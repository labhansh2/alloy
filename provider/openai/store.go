package openai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"alloy/internal/credentialstore"
)

// diskStore adapts opaque encrypted storage to OpenAI's credential schema and
// registration identity. These legacy storage identifiers must not change.
type diskStore struct {
	dir     string
	backend *credentialstore.Store
}

func newStore(account string) (*diskStore, error) {
	if credentialstore.ValidateLabel(account) != nil {
		return nil, errors.New("invalid OpenAI account label")
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return nil, errors.New("cannot locate OpenAI credential storage")
	}

	dir := filepath.Join(base, "alloy", "providers", "openai")
	digest := sha256.Sum256([]byte(dir))
	keyName := "encryption-key-" + hex.EncodeToString(digest[:16])

	backend, err := credentialstore.New(credentialstore.Config{
		Directory:      dir,
		Account:        account,
		KeyringService: "alloy.openai",
		KeyringAccount: keyName,
		AssociatedData: "alloy.openai.v1:" + account,
	})
	if err != nil {
		return nil, err
	}

	return &diskStore{dir: dir, backend: backend}, nil
}

func (s *diskStore) locked(ctx context.Context, work func(credentialRecord) error) error {
	return s.backend.WithRecord(ctx, func(record *credentialstore.Record) error {
		return work(&diskRecord{dir: s.dir, record: record})
	})
}

type diskRecord struct {
	dir    string
	record *credentialstore.Record
}

func (s *diskRecord) load() (*credential, error) {
	plain, err := s.record.Load()
	if err != nil {
		return nil, err
	}

	var saved credential
	if json.Unmarshal(plain, &saved) != nil {
		return nil, errors.New("invalid OpenAI credential record")
	}

	return &saved, nil
}

func (s *diskRecord) save(saved *credential) error {
	plain, err := json.Marshal(saved)
	if err != nil {
		return errors.New("cannot encode OpenAI credentials")
	}

	return s.record.Save(plain)
}

func (s *diskRecord) hostID() (string, error) {
	path := filepath.Join(s.dir, "host-id")

	value, err := credentialstore.ReadPrivateFile(path)
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

	return id, credentialstore.WritePrivateFile(path, []byte(id))
}
