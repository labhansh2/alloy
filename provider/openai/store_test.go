package openai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

func TestEncryptedStoreAndStableHost(t *testing.T) {
	var master string

	s := &diskStore{dir: filepath.Join(t.TempDir(), "credentials"), account: "test",
		keyGet: func() (string, error) {
			if master == "" {
				return "", keyring.ErrNotFound
			}

			return master, nil
		},
		keySet: func(key string) error { master = key; return nil },
	}

	err := s.locked(context.Background(), func() error {
		first, err := s.hostID()
		if err != nil {
			return err
		}

		second, err := s.hostID()
		if err != nil || first != second {
			t.Fatal("host ID changed")
		}

		saved := &credential{Mode: "token", Token: &oauth2.Token{AccessToken: "seeded-secret"}}
		if err := s.save(saved); err != nil {
			return err
		}

		encoded, err := os.ReadFile(filepath.Join(s.dir, "test.enc"))
		if err != nil {
			return err
		}

		if strings.Contains(string(encoded), "seeded-secret") {
			t.Fatal("credentials were stored in plaintext")
		}

		loaded, err := s.load()
		if err != nil || loaded.Token.AccessToken != "seeded-secret" {
			t.Fatalf("could not unlock credentials: %v", err)
		}

		encoded[len(encoded)-1] ^= 1
		if err := os.WriteFile(filepath.Join(s.dir, "test.enc"), encoded, 0600); err != nil {
			return err
		}

		if _, err := s.load(); err == nil {
			t.Fatal("tampered ciphertext was accepted")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNoPlaintextFallback(t *testing.T) {
	s := &diskStore{dir: t.TempDir(), account: "test",
		keyGet: func() (string, error) { return "", errors.New("unavailable") }, keySet: func(string) error { return errors.New("unavailable") },
	}
	if err := s.save(&credential{Token: &oauth2.Token{AccessToken: "secret"}}); err == nil {
		t.Fatal("unavailable keychain was ignored")
	}

	if _, err := os.Stat(filepath.Join(s.dir, "test.enc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed credential save wrote a file")
	}
}

func TestRejectsPermissiveAndSymlinkCredentialFiles(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "credentials")
	if err := os.WriteFile(path, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := privateRead(path); err == nil {
		t.Fatal("permissive credential file accepted")
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}

	if _, err := privateRead(link); err == nil {
		t.Fatal("symlink credential file accepted")
	}
}
