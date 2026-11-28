package credentials

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestNamespacedStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)

	namespaces := []Namespace{
		{Server: "home", Integration: "provider.google", Account: "work"},
		{Server: "other", Integration: "provider.google", Account: "work"},
		{Server: "home", Integration: "cli.example", Account: "work"},
		{Server: "home", Integration: "provider.google", Account: "personal"},
		{Server: "Home", Integration: "provider.google", Account: "work"},
	}
	stores := make([]Store, len(namespaces))

	for i, namespace := range namespaces {
		store, err := NewStore(namespace)
		if err != nil {
			t.Fatal(err)
		}

		stores[i] = store
	}

	base, _ := os.UserConfigDir()
	if _, err := os.Stat(filepath.Join(base, "alloy")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("construction was not lazy")
	}

	for i, store := range stores {
		err := store.WithRecord(context.Background(), func(record Record) error {
			if _, err := record.Load(); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("namespace could read another namespace's record")
			}

			return record.Save([]byte{byte(i), 0, 255})
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Reopening the same namespace finds the same key and ciphertext, even
	// though no credentials are held in memory by the new handle.
	for i, namespace := range namespaces {
		store, err := NewStore(namespace)
		if err != nil {
			t.Fatal(err)
		}

		var escaped Record

		err = store.WithRecord(context.Background(), func(record Record) error {
			escaped = record

			value, err := record.Load()
			if err != nil || !bytes.Equal(value, []byte{byte(i), 0, 255}) {
				t.Fatal("namespaced record did not persist")
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := escaped.Load(); !errors.Is(err, ErrRecordClosed) {
			t.Fatal("public record could be used outside its callback")
		}
	}
}

func TestNamespaceValidation(t *testing.T) {
	for _, namespace := range []Namespace{
		{},
		{Server: "..", Integration: "google", Account: "work"},
		{Server: "home", Integration: "../google", Account: "work"},
		{Server: "home", Integration: "google", Account: ""},
		{Server: "home", Integration: "google", Account: "bad/account"},
	} {
		if _, err := NewStore(namespace); err == nil {
			t.Fatal("invalid namespace accepted")
		}
	}
}
