// Package credentialstore implements encrypted local records. Public extension
// contracts live in alloy/credentials; integration policy does not belong here.
package credentialstore

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/zalando/go-keyring"
)

const (
	maxFileBytes = 1 << 20
	nonceSize    = 12
	tagSize      = 16
)

// ErrRecordClosed means a record was used outside its locked callback.
var ErrRecordClosed = errors.New("credential record is outside its locked callback")

// Config fixes a record's disk, keyring, and authenticated-encryption identity.
// It is internal so legacy integrations can preserve their exact storage layout.
type Config struct {
	Directory      string
	Account        string
	KeyringService string
	KeyringAccount string
	AssociatedData string
}

// Store contains configuration, never cached credential values or encryption keys.
type Store struct {
	config Config
	gate   chan struct{}
	keyGet func() (string, error)
	keySet func(string) error
}

// New validates configuration without touching files or the OS keyring.
func New(config Config) (*Store, error) {
	if !filepath.IsAbs(config.Directory) || ValidateLabel(config.Account) != nil ||
		config.KeyringService == "" || config.KeyringAccount == "" || config.AssociatedData == "" {
		return nil, errors.New("invalid credential storage configuration")
	}

	return &Store{
		config: config,
		gate:   make(chan struct{}, 1),
		keyGet: func() (string, error) { return keyring.Get(config.KeyringService, config.KeyringAccount) },
		keySet: func(key string) error { return keyring.Set(config.KeyringService, config.KeyringAccount, key) },
	}, nil
}

// ValidateLabel rejects traversal and nonportable label characters.
func ValidateLabel(value string) error {
	if value == "" || value == "." || value == ".." || len(value) > 128 {
		return errors.New("invalid credential namespace label")
	}

	for _, c := range value {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.", c) {
			return errors.New("invalid credential namespace label")
		}
	}

	return nil
}

// WithRecord locks the entire read/refresh/save operation, across goroutines and
// processes. Cancellation bounds lock acquisition, not arbitrary callback code.
// Saves take effect immediately; a callback error does not roll them back.
func (s *Store) WithRecord(ctx context.Context, work func(*Record) error) error {
	if work == nil {
		return errors.New("credential callback is required")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	defer func() { <-s.gate }()

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.MkdirAll(s.config.Directory, 0700); err != nil {
		return errors.New("cannot create credential directory")
	}

	info, err := os.Lstat(s.config.Directory)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("credential directory must be private (0700)")
	}

	lockPath := filepath.Join(s.config.Directory, "credentials.lock")
	if info, err := os.Lstat(lockPath); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return errors.New("credential lock must be regular and private")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("cannot inspect credential lock")
	}

	lock := flock.New(lockPath, flock.SetPermissions(0600))
	defer lock.Close()

	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil || !ok {
		if err := ctx.Err(); err != nil {
			return err
		}

		return errors.New("cannot acquire credential lock")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	record := &Record{store: s, active: true}
	defer record.close()

	return work(record)
}

// Record is an opaque byte record leased for the duration of WithRecord.
type Record struct {
	mu     sync.Mutex
	store  *Store
	active bool
}

func (r *Record) close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.active = false
}

// Load returns a fresh plaintext buffer. Missing files report os.ErrNotExist.
func (r *Record) Load() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.active {
		return nil, ErrRecordClosed
	}

	encoded, err := ReadPrivateFile(r.store.path())
	if err != nil {
		return nil, err
	}

	aead, err := r.store.aead(false)
	if err != nil {
		return nil, err
	}

	if len(encoded) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("credential record is damaged; explicit recovery is required")
	}

	plain, err := aead.Open(nil, encoded[:aead.NonceSize()], encoded[aead.NonceSize():], []byte(r.store.config.AssociatedData))
	if err != nil {
		return nil, errors.New("credentials cannot be decrypted; recover the OS key or explicitly reset this account")
	}

	return plain, nil
}

// Save encrypts and atomically replaces the record; it never writes plaintext.
// Records, including encryption overhead, are bounded to one MiB.
func (r *Record) Save(plain []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.active {
		return ErrRecordClosed
	}

	if len(plain) > maxFileBytes-nonceSize-tagSize {
		return errors.New("credential record is too large")
	}

	aead, err := r.store.aead(true)
	if err != nil {
		return err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return errors.New("cannot generate credential nonce")
	}

	encoded := aead.Seal(nonce, nonce, plain, []byte(r.store.config.AssociatedData))

	return WritePrivateFile(r.store.path(), encoded)
}

func (s *Store) path() string {
	return filepath.Join(s.config.Directory, s.config.Account+".enc")
}

func (s *Store) aead(create bool) (cipher.AEAD, error) {
	encoded, err := s.keyGet()
	if errors.Is(err, keyring.ErrNotFound) && create {
		// A missing OS key is not a reason to strand existing records by
		// silently creating a replacement encryption key.
		entries, readErr := os.ReadDir(s.config.Directory)
		if readErr != nil {
			return nil, errors.New("cannot inspect credential directory")
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".enc") {
				return nil, errors.New("credential encryption key is missing; explicit recovery is required")
			}
		}

		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, errors.New("cannot generate credential encryption key")
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
		return nil, errors.New("cannot initialize credential encryption")
	}

	return cipher.NewGCM(block)
}
