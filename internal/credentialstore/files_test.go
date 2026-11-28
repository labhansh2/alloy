package credentialstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectsPermissiveSymlinkAndOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials")

	if err := os.WriteFile(path, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadPrivateFile(path); err == nil {
		t.Fatal("permissive credential file accepted")
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadPrivateFile(link); err == nil {
		t.Fatal("symlink credential file accepted")
	}

	if _, err := ReadPrivateFile(dir); err == nil {
		t.Fatal("directory accepted as a credential file")
	}

	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, make([]byte, maxFileBytes+1), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadPrivateFile(path); err == nil {
		t.Fatal("oversized credential file accepted")
	}
}

func TestRejectsUnsafeDirectoryAndLock(t *testing.T) {
	for _, kind := range []string{"permissive-directory", "symlink-directory", "symlink-lock", "permissive-lock"} {
		t.Run(kind, func(t *testing.T) {
			config := testConfig(t)
			store := newTestStore(t, config, &memoryKeyring{})

			target := filepath.Join(t.TempDir(), "target")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}

			if kind == "symlink-directory" {
				if err := os.Symlink(target, config.Directory); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(config.Directory, 0700); err != nil {
					t.Fatal(err)
				}

				switch kind {
				case "permissive-directory":
					if err := os.Chmod(config.Directory, 0755); err != nil {
						t.Fatal(err)
					}
				case "symlink-lock":
					if err := os.Symlink(filepath.Join(target, "lock"), filepath.Join(config.Directory, "credentials.lock")); err != nil {
						t.Fatal(err)
					}
				case "permissive-lock":
					if err := os.WriteFile(filepath.Join(config.Directory, "credentials.lock"), nil, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}

			if err := store.WithRecord(context.Background(), func(*Record) error {
				t.Error("unsafe storage callback ran")
				return nil
			}); err == nil {
				t.Fatal("unsafe storage was accepted")
			}
		})
	}
}
