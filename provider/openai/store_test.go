package openai

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

func isolatedStore(t *testing.T) *diskStore {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)

	store, err := newStore("test")
	if err != nil {
		t.Fatal(err)
	}

	return store
}

func TestEncryptedStoreAndStableHost(t *testing.T) {
	s := isolatedStore(t)

	err := s.locked(context.Background(), func(record credentialRecord) error {
		first, err := record.hostID()
		if err != nil {
			return err
		}

		second, err := record.hostID()
		if err != nil || first != second {
			t.Fatal("host ID changed")
		}

		saved := &credential{Mode: "token", Token: &oauth2.Token{AccessToken: "seeded-secret"}}
		if err := record.save(saved); err != nil {
			return err
		}

		encoded, err := os.ReadFile(filepath.Join(s.dir, "test.enc"))
		if err != nil {
			return err
		}

		if strings.Contains(string(encoded), "seeded-secret") {
			t.Fatal("credentials were stored in plaintext")
		}

		loaded, err := record.load()
		if err != nil || loaded.Token.AccessToken != "seeded-secret" {
			t.Fatalf("could not unlock credentials: %v", err)
		}

		encoded[len(encoded)-1] ^= 1
		if err := os.WriteFile(filepath.Join(s.dir, "test.enc"), encoded, 0600); err != nil {
			return err
		}

		if _, err := record.load(); err == nil {
			t.Fatal("tampered ciphertext was accepted")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNoPlaintextFallback(t *testing.T) {
	s := isolatedStore(t)

	keyring.MockInitWithError(errors.New("unavailable"))

	if err := s.locked(context.Background(), func(record credentialRecord) error {
		return record.save(&credential{Token: &oauth2.Token{AccessToken: "secret"}})
	}); err == nil {
		t.Fatal("unavailable keychain was ignored")
	}

	if _, err := os.Stat(filepath.Join(s.dir, "test.enc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed credential save wrote a file")
	}
}

func TestLegacyCredentialCompatibility(t *testing.T) {
	s := isolatedStore(t)
	base, _ := os.UserConfigDir()

	legacyDir := filepath.Join(base, "alloy", "providers", "openai")
	if s.dir != legacyDir {
		t.Fatal("OpenAI credential directory changed")
	}

	if err := os.MkdirAll(legacyDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Build a fixture independently using the pre-extraction file format,
	// keyring lookup, AAD, and host identity location.
	key := []byte("0123456789abcdef0123456789abcdef")
	digest := sha256.Sum256([]byte(legacyDir))

	keyName := "encryption-key-" + hex.EncodeToString(digest[:16])
	if err := keyring.Set("alloy.openai", keyName, base64.StdEncoding.EncodeToString(key)); err != nil {
		t.Fatal(err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}

	plain := []byte(`{"mode":"browser","client_id":"registered-client","subject":"same-user","scopes":"openid resource.invoke chatgpt.tokens.use.direct","token":{"access_token":"legacy-access","refresh_token":"legacy-refresh","token_type":"Bearer"}}`)
	nonce := []byte("legacy-nonce")
	encoded := aead.Seal(append([]byte(nil), nonce...), nonce, plain, []byte("alloy.openai.v1:test"))

	path := filepath.Join(legacyDir, "test.enc")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}

	host := "urn:uuid:12345678-1234-4234-8234-123456789abc"
	if err := os.WriteFile(filepath.Join(legacyDir, "host-id"), []byte(host), 0600); err != nil {
		t.Fatal(err)
	}

	if err := s.locked(context.Background(), func(record credentialRecord) error {
		saved, err := record.load()
		if err != nil || saved.Token.AccessToken != "legacy-access" || saved.Token.RefreshToken != "legacy-refresh" || saved.Subject != "same-user" {
			t.Fatalf("legacy credentials were not preserved: %v", err)
		}

		id, err := record.hostID()
		if err != nil || id != host {
			t.Fatal("legacy registration host identity changed")
		}

		saved.Token.AccessToken = "renewed-access"

		return record.save(saved)
	}); err != nil {
		t.Fatal(err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := aead.Open(nil, updated[:aead.NonceSize()], updated[aead.NonceSize():], []byte("alloy.openai.v1:test"))
	if err != nil {
		t.Fatal("new records no longer use the legacy encryption format")
	}

	var saved credential
	if err := json.Unmarshal(decrypted, &saved); err != nil || saved.Token.AccessToken != "renewed-access" {
		t.Fatal("legacy reader could not decode updated credentials")
	}

	storedKey, err := keyring.Get("alloy.openai", keyName)
	if err != nil || storedKey != base64.StdEncoding.EncodeToString(key) {
		t.Fatal("legacy encryption key was replaced")
	}
}

func TestInvalidAccountsAndSchema(t *testing.T) {
	for _, account := range []string{"", ".", "..", "../escape", "bad account"} {
		if _, err := newStore(account); err == nil {
			t.Fatal("invalid OpenAI account accepted")
		}
	}

	s := isolatedStore(t)
	if err := s.locked(context.Background(), func(record credentialRecord) error {
		if err := record.(*diskRecord).record.Save([]byte("not-json")); err != nil {
			return err
		}

		if _, err := record.load(); err == nil {
			t.Fatal("opaque storage bypassed OpenAI credential schema validation")
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
