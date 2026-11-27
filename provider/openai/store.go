package openai

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/zalando/go-keyring"
)

// Token records are atomically encrypted together; the encryption key lives in
// the OS credential store, not beside the file. Refreshes share a process lock.
type diskStore struct {
	dir     string
	account string
	keyGet  func() (string, error)
	keySet  func(string) error
}

func newStore(account string) (*diskStore, error) {
	if account == "" || account == "." || account == ".." || len(account) > 128 {
		return nil, errors.New("invalid OpenAI account label")
	}

	for _, c := range account {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.", c) {
			return nil, errors.New("invalid OpenAI account label")
		}
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return nil, errors.New("cannot locate OpenAI credential storage")
	}

	dir := filepath.Join(base, "alloy", "providers", "openai")
	digest := sha256.Sum256([]byte(dir))
	keyName := "encryption-key-" + hex.EncodeToString(digest[:16])

	return &diskStore{
		dir: dir, account: account,
		keyGet: func() (string, error) { return keyring.Get("alloy.openai", keyName) },
		keySet: func(key string) error { return keyring.Set("alloy.openai", keyName, key) },
	}, nil
}

func (s *diskStore) locked(ctx context.Context, work func() error) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return errors.New("cannot create OpenAI credential directory")
	}

	info, err := os.Lstat(s.dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("OpenAI credential directory must be private (0700)")
	}

	lockPath := filepath.Join(s.dir, "credentials.lock")
	if info, err := os.Lstat(lockPath); err == nil && !info.Mode().IsRegular() {
		return errors.New("invalid OpenAI credential lock")
	}

	lock := flock.New(lockPath, flock.SetPermissions(0600))

	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil || !ok {
		return errors.New("cannot acquire OpenAI credential lock")
	}
	defer lock.Close()

	return work()
}

func (s *diskStore) load() (*credential, error) {
	encoded, err := privateRead(filepath.Join(s.dir, s.account+".enc"))
	if err != nil {
		return nil, err
	}

	aead, err := s.aead(false)
	if err != nil {
		return nil, err
	}

	if len(encoded) < aeadNonceSize {
		return nil, errors.New("OpenAI credential record is damaged; explicit recovery is required")
	}

	plain, err := aead.Open(nil, encoded[:aeadNonceSize], encoded[aeadNonceSize:], []byte("alloy.openai.v1:"+s.account))
	if err != nil {
		return nil, errors.New("OpenAI credentials cannot be decrypted; recover the OS key or explicitly reset this account")
	}

	var saved credential
	if json.Unmarshal(plain, &saved) != nil {
		return nil, errors.New("invalid OpenAI credential record")
	}

	return &saved, nil
}

const aeadNonceSize = 12

func (s *diskStore) aead(create bool) (cipher.AEAD, error) {
	encoded, err := s.keyGet()
	if errors.Is(err, keyring.ErrNotFound) && create {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}

		encoded = base64.StdEncoding.EncodeToString(key)
		err = s.keySet(encoded)
	}

	if err != nil {
		return nil, errors.New("OS credential store unavailable; no plaintext fallback")
	}

	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("invalid credential encryption key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

func (s *diskStore) save(saved *credential) error {
	plain, err := json.Marshal(saved)
	if err != nil {
		return errors.New("cannot encode OpenAI credentials")
	}

	aead, err := s.aead(true)
	if err != nil {
		return err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}

	encoded := aead.Seal(nonce, nonce, plain, []byte("alloy.openai.v1:"+s.account))

	return privateWrite(filepath.Join(s.dir, s.account+".enc"), encoded)
}

func (s *diskStore) hostID() (string, error) {
	path := filepath.Join(s.dir, "host-id")

	value, err := privateRead(path)
	if err == nil {
		if !strings.HasPrefix(string(value), "urn:uuid:") {
			return "", errors.New("invalid OpenAI host identity")
		}

		return string(value), nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	id := "urn:uuid:" + h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]

	return id, privateWrite(path, []byte(id))
}

func privateRead(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 1<<20 {
		return nil, errors.New("credential files must be regular, private, and bounded")
	}

	return os.ReadFile(path)
}

func privateWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".credentials-*")
	if err != nil {
		return errors.New("cannot create credential file")
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return errors.New("cannot write credential file")
	}

	if err := f.Sync(); err != nil {
		return errors.New("cannot sync credential file")
	}

	if err := f.Close(); err != nil {
		return errors.New("cannot close credential file")
	}

	if err := os.Rename(f.Name(), path); err != nil {
		return errors.New("cannot replace credential file")
	}

	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return errors.New("cannot open credential directory for sync")
	}
	defer dir.Close()

	return dir.Sync()
}
