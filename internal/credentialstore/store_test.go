package credentialstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/zalando/go-keyring"
)

type memoryKeyring struct {
	mu   sync.Mutex
	keys map[string]string
	err  error
	sets int
}

func newTestStore(t *testing.T, config Config, keys *memoryKeyring) *Store {
	t.Helper()

	s, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	s.keyGet = func() (string, error) {
		keys.mu.Lock()
		defer keys.mu.Unlock()

		if keys.err != nil {
			return "", keys.err
		}

		value, ok := keys.keys[config.KeyringService+":"+config.KeyringAccount]
		if !ok {
			return "", keyring.ErrNotFound
		}

		return value, nil
	}
	s.keySet = func(value string) error {
		keys.mu.Lock()
		defer keys.mu.Unlock()

		if keys.err != nil {
			return keys.err
		}

		if keys.keys == nil {
			keys.keys = make(map[string]string)
		}

		keys.keys[config.KeyringService+":"+config.KeyringAccount] = value
		keys.sets++

		return nil
	}

	return s
}

func testConfig(t *testing.T) Config {
	t.Helper()

	return Config{
		Directory:      filepath.Join(t.TempDir(), "credentials"),
		Account:        "test",
		KeyringService: "test.credentials",
		KeyringAccount: "test-key",
		AssociatedData: "test.credentials.v1:test",
	}
}

func TestOpaqueEncryptedRecord(t *testing.T) {
	config := testConfig(t)
	keys := &memoryKeyring{}
	store := newTestStore(t, config, keys)

	if _, err := os.Stat(config.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("construction created a credential directory")
	}

	plain := []byte("seeded-secret\x00\xff-not-json")

	var escaped *Record

	err := store.WithRecord(context.Background(), func(record *Record) error {
		escaped = record

		if _, err := record.Load(); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("absent record did not report os.ErrNotExist")
		}

		if keys.sets != 0 {
			t.Fatal("reading a missing record initialized a key")
		}

		if err := record.Save(plain); err != nil {
			return err
		}

		loaded, err := record.Load()
		if err != nil || !bytes.Equal(plain, loaded) {
			t.Fatal("opaque bytes did not round-trip")
		}

		loaded[0] ^= 1

		again, err := record.Load()
		if err != nil || !bytes.Equal(plain, again) {
			t.Fatal("caller mutated stored data")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := os.ReadFile(store.path())
	if err != nil || bytes.Contains(encoded, []byte("seeded-secret")) {
		t.Fatal("credential was not encrypted")
	}

	for _, path := range []string{store.path(), filepath.Join(config.Directory, "credentials.lock")} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("credential or lock file is not private")
		}
	}

	info, err := os.Stat(config.Directory)
	if err != nil || info.Mode().Perm() != 0700 || keys.sets != 1 {
		t.Fatal("directory permissions or encryption-key lifecycle changed")
	}

	if _, err := escaped.Load(); !errors.Is(err, ErrRecordClosed) {
		t.Fatal("escaped record remained readable")
	}

	if err := escaped.Save([]byte("escaped-secret")); !errors.Is(err, ErrRecordClosed) {
		t.Fatal("escaped record remained writable")
	}
}

func TestNoPlaintextFallback(t *testing.T) {
	for _, failure := range []string{"read", "write", "malformed"} {
		t.Run(failure, func(t *testing.T) {
			store := newTestStore(t, testConfig(t), &memoryKeyring{})

			switch failure {
			case "read":
				store.keyGet = func() (string, error) { return "", errors.New("unavailable: seeded-secret") }
			case "write":
				store.keySet = func(string) error { return errors.New("unavailable: seeded-secret") }
			case "malformed":
				store.keyGet = func() (string, error) { return "seeded-secret", nil }
			}

			err := store.WithRecord(context.Background(), func(record *Record) error {
				return record.Save([]byte("seeded-secret"))
			})
			if err == nil || strings.Contains(err.Error(), "seeded-secret") {
				t.Fatal("keyring error ignored or leaked a secret")
			}

			if _, err := os.Stat(store.path()); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed save wrote a credential file")
			}
		})
	}
}

func TestMissingKeyDoesNotOverwriteOrStrandRecords(t *testing.T) {
	keys := &memoryKeyring{}
	config := testConfig(t)
	store := newTestStore(t, config, keys)

	if err := store.WithRecord(context.Background(), func(record *Record) error {
		return record.Save([]byte("original"))
	}); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(store.path())
	keys.keys = nil

	err := store.WithRecord(context.Background(), func(record *Record) error {
		return record.Save([]byte("replacement"))
	})
	if err == nil || keys.sets != 1 {
		t.Fatal("lost encryption key was silently replaced")
	}

	config.Account = "another-account"
	config.AssociatedData = "test.credentials.v1:another-account"

	other := newTestStore(t, config, keys)
	if err := other.WithRecord(context.Background(), func(record *Record) error {
		return record.Save([]byte("another"))
	}); err == nil || keys.sets != 1 {
		t.Fatal("another account silently replaced the shared directory key")
	}

	after, _ := os.ReadFile(store.path())
	if !bytes.Equal(before, after) {
		t.Fatal("failed recovery overwrote existing ciphertext")
	}
}

func TestRejectsTamperingAndNamespaceSubstitution(t *testing.T) {
	config := testConfig(t)
	keys := &memoryKeyring{}
	store := newTestStore(t, config, keys)

	if err := store.WithRecord(context.Background(), func(record *Record) error {
		return record.Save([]byte("seeded-secret"))
	}); err != nil {
		t.Fatal(err)
	}

	encoded, _ := os.ReadFile(store.path())

	for _, changed := range [][]byte{{}, encoded[:nonceSize], append([]byte(nil), encoded...)} {
		if len(changed) > nonceSize {
			changed[len(changed)-1] ^= 1
		}

		if err := os.WriteFile(store.path(), changed, 0600); err != nil {
			t.Fatal(err)
		}

		if err := store.WithRecord(context.Background(), func(record *Record) error {
			_, err := record.Load()
			return err
		}); err == nil {
			t.Fatal("damaged or tampered record was accepted")
		}
	}

	if err := os.WriteFile(store.path(), encoded, 0600); err != nil {
		t.Fatal(err)
	}

	config.AssociatedData = "different-server:integration:account"

	other := newTestStore(t, config, keys)
	if err := other.WithRecord(context.Background(), func(record *Record) error {
		_, err := record.Load()
		return err
	}); err == nil {
		t.Fatal("ciphertext accepted under another authenticated namespace")
	}
}

func TestRecordBoundsAndImmediateSaves(t *testing.T) {
	store := newTestStore(t, testConfig(t), &memoryKeyring{})
	callbackError := errors.New("callback stopped")

	err := store.WithRecord(context.Background(), func(record *Record) error {
		if err := record.Save(make([]byte, maxFileBytes-nonceSize-tagSize+1)); err == nil {
			t.Fatal("oversized plaintext accepted")
		}

		if err := record.Save([]byte("saved")); err != nil {
			return err
		}

		return callbackError
	})
	if !errors.Is(err, callbackError) {
		t.Fatal("callback error was lost")
	}

	if err := store.WithRecord(context.Background(), func(record *Record) error {
		plain, err := record.Load()
		if err != nil || string(plain) != "saved" {
			t.Fatal("completed save was unexpectedly rolled back")
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentReadModifySaveAcrossHandles(t *testing.T) {
	config := testConfig(t)
	keys := &memoryKeyring{}
	handles := []*Store{newTestStore(t, config, keys), newTestStore(t, config, keys)}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 8 {
				if err := handles[worker%2].WithRecord(ctx, func(record *Record) error {
					data, err := record.Load()
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						return err
					}

					value := 0
					if err == nil {
						value, err = strconv.Atoi(string(data))
						if err != nil {
							return err
						}
					}

					return record.Save([]byte(strconv.Itoa(value + 1)))
				}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}

	wg.Wait()

	if err := handles[0].WithRecord(ctx, func(record *Record) error {
		value, err := record.Load()
		if err != nil || string(value) != "64" {
			t.Error("read/modify/save operations were not serialized")
		}

		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledLockAcquisition(t *testing.T) {
	store := newTestStore(t, testConfig(t), &memoryKeyring{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := store.WithRecord(ctx, func(*Record) error { t.Fatal("canceled callback ran"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled acquisition did not preserve context error")
	}

	if _, err := os.Stat(store.config.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled operation created storage")
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- store.WithRecord(context.Background(), func(*Record) error {
			close(entered)
			<-release

			return nil
		})
	}()

	<-entered

	wait, stop := context.WithTimeout(context.Background(), 50*time.Millisecond)
	err := store.WithRecord(wait, func(*Record) error { t.Error("blocked callback ran"); return nil })

	stop()
	close(release)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("in-process wait ignored cancellation")
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	lock := flock.New(filepath.Join(store.config.Directory, "credentials.lock"))
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	wait, stop = context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer stop()

	if err := store.WithRecord(wait, func(*Record) error { t.Error("locked callback ran"); return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("file-lock wait ignored cancellation")
	}
}

func TestInvalidConfigurationAndLazyConstruction(t *testing.T) {
	for _, label := range []string{"", ".", "..", "../escape", "with/slash", "with space", "🙂", strings.Repeat("a", 129)} {
		config := testConfig(t)
		config.Account = label

		if _, err := New(config); err == nil {
			t.Error("invalid account accepted")
		}
	}

	config := testConfig(t)

	config.Directory = "relative"
	if _, err := New(config); err == nil {
		t.Fatal("relative storage directory accepted")
	}

	store := newTestStore(t, testConfig(t), &memoryKeyring{})
	if err := store.WithRecord(context.Background(), nil); err == nil {
		t.Fatal("nil callback accepted")
	}

	if _, err := os.Stat(store.config.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid operation created storage")
	}

	// A malformed base64 encoding and a well-formed, wrong-length key both fail.
	for _, key := range []string{"invalid-key", base64.StdEncoding.EncodeToString([]byte("short"))} {
		store.keyGet = func() (string, error) { return key, nil }

		if err := store.WithRecord(context.Background(), func(record *Record) error {
			return record.Save([]byte("secret"))
		}); err == nil {
			t.Fatal("invalid encryption key accepted")
		}
	}
}
