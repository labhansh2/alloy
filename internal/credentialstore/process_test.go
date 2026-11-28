package credentialstore

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestCredentialLockHelperProcess(t *testing.T) {
	dir := os.Getenv("ALLOY_TEST_CREDENTIAL_LOCK_DIR")
	if dir == "" {
		return
	}

	lock := flock.New(filepath.Join(dir, "credentials.lock"))
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	if _, err := io.WriteString(os.Stdout, "locked\n"); err != nil {
		t.Fatal(err)
	}

	if _, err := bufio.NewReader(os.Stdin).ReadByte(); err != nil {
		t.Fatal(err)
	}
}

func TestCrossProcessLock(t *testing.T) {
	config := testConfig(t)
	if err := os.Mkdir(config.Directory, 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(config.Directory, "credentials.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCredentialLockHelperProcess$")

	command.Env = append(os.Environ(), "ALLOY_TEST_CREDENTIAL_LOCK_DIR="+config.Directory)

	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := command.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = input.Close()

		if err := command.Wait(); err != nil {
			t.Error(err)
		}
	}()

	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatal("child process did not acquire the credential lock")
	}

	store := newTestStore(t, config, &memoryKeyring{})
	wait, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	err = store.WithRecord(wait, func(*Record) error {
		t.Error("another process's credential lock was bypassed")
		return nil
	})

	stop()

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("cross-process acquisition did not honor cancellation")
	}

	if _, err := input.Write([]byte{'\n'}); err != nil {
		t.Fatal(err)
	}

	// Wait for the released OS lock without polling; WithRecord acquires it.
	if err := store.WithRecord(ctx, func(*Record) error { return nil }); err != nil {
		t.Fatal(err)
	}
}
