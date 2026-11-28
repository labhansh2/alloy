// Package credentials provides opaque, locked credential records for Alloy
// integrations. It does not define OAuth flows, token schemas, or login policy.
package credentials

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"alloy/internal/credentialstore"
)

// Namespace explicitly separates servers, integrations, and account profiles.
// Labels are nonsecret metadata: 1–128 ASCII letters, digits, '-', '_', or '.',
// excluding "." and "..". No implicit shared server or account is selected.
type Namespace struct {
	Server      string
	Integration string
	Account     string
}

// Store serializes access to one opaque credential record. Implementations must
// hold the lock for the entire callback, including any refresh and subsequent
// save. The default store supports concurrent calls and cross-process locking.
// Do not recursively acquire the same server/integration lock, even for another account.
type Store interface {
	WithRecord(context.Context, func(Record) error) error
}

// Record is usable only during its Store.WithRecord callback. Load returns a fresh
// byte slice and reports os.ErrNotExist for an absent record. Save takes effect
// immediately: returning a callback error does not roll back a completed save.
type Record interface {
	Load() ([]byte, error)
	Save([]byte) error
}

// ErrRecordClosed means a default-store record escaped its locked callback.
var ErrRecordClosed = credentialstore.ErrRecordClosed

// NewStore creates an OS-keyring-backed encrypted file store. Construction
// validates its namespace but creates no files and does not access the keyring.
// File paths are hashed beneath the user config directory at alloy/credentials/v1;
// payloads are opaque bytes, bounded to one MiB including encryption overhead.
// Missing OS keyring access fails closed: there is no plaintext fallback.
func NewStore(namespace Namespace) (Store, error) {
	for _, label := range []string{namespace.Server, namespace.Integration, namespace.Account} {
		if err := credentialstore.ValidateLabel(label); err != nil {
			return nil, err
		}
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return nil, errors.New("cannot locate credential storage")
	}

	dir := filepath.Join(base, "alloy", "credentials", "v1",
		"server-"+digest(namespace.Server), "integration-"+digest(namespace.Integration))
	// Labels exclude ':', so this versioned tuple has no ambiguous boundaries.
	identity := namespace.Server + ":" + namespace.Integration + ":" + namespace.Account

	backend, err := credentialstore.New(credentialstore.Config{
		Directory:      dir,
		Account:        "account-" + digest(namespace.Account),
		KeyringService: "alloy.credentials",
		KeyringAccount: "encryption-key-" + digest(dir),
		AssociatedData: "alloy.credentials.v1:" + identity,
	})
	if err != nil {
		return nil, err
	}

	return &localStore{backend: backend}, nil
}

type localStore struct {
	backend *credentialstore.Store
}

func (s *localStore) WithRecord(ctx context.Context, work func(Record) error) error {
	if work == nil {
		return errors.New("credential callback is required")
	}

	return s.backend.WithRecord(ctx, func(record *credentialstore.Record) error {
		return work(record)
	})
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
