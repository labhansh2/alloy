// Command prototype-server composes the CLI/event, Codex, Notion and Browse
// examples on one loopback listener. Provider policy belongs to this command.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofrs/flock"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	stateDir := flag.String("state-dir", defaultStateDir(configDir), "private server state directory outside the repo")

	defaultAddr := os.Getenv("ALLOY_PROTOTYPE_ADDR")
	if defaultAddr == "" {
		defaultAddr = "127.0.0.1:0"
	}

	addr := flag.String("listen", defaultAddr, "loopback listener shared by all gateways")
	withCodex := flag.Bool("codex", true, "expose the experimental host-authenticated Codex connection")
	withNotion := flag.Bool("notion", true, "connect and expose Notion MCP search")
	withBrowse := flag.Bool("browse", true, "connect Browse MCP for the browser node (requires Codex)")
	reauthorize := flag.Bool("reauthorize", false, "request fresh Notion consent instead of reusing its cache")

	flag.Parse()

	if *withBrowse && !*withCodex {
		return errors.New("the browser example requires -codex=true; disable -browse too for a server without inference")
	}

	host, _, err := net.SplitHostPort(*addr)
	if err != nil || (host != "127.0.0.1" && host != "::1" && host != "localhost") {
		return errors.New("prototype listener must use a loopback host")
	}

	if err := os.MkdirAll(*stateDir, 0700); err != nil {
		return err
	}

	info, err := os.Lstat(*stateDir)
	if err != nil {
		return err
	}

	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("state directory must be private (0700)")
	}

	lock, err := acquireStateLock(*stateDir)
	if err != nil {
		return err
	}
	defer lock.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}

	baseURL := "http://" + listener.Addr().String()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	httpServer := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	done := make(chan error, 1)

	go func() { done <- httpServer.Serve(listener) }()

	defer func() { _ = httpServer.Close(); <-done }()

	if err := registerCLI(mux, baseURL, *stateDir); err != nil {
		return err
	}

	if err := registerEvents(mux, baseURL, *stateDir); err != nil {
		return err
	}

	browserCodexToken := ""
	if *withBrowse {
		browserCodexToken = rand.Text()
	}

	if *withCodex {
		if err := registerCodex(mux, baseURL, *stateDir, browserCodexToken); err != nil {
			return err
		}
	}

	if *withNotion {
		cleanup, err := registerNotion(ctx, mux, baseURL, *stateDir, *reauthorize)
		if err != nil {
			return err
		}
		defer cleanup()
	}

	if *withBrowse {
		cleanup, err := registerBrowse(ctx, mux, baseURL, *stateDir, browserCodexToken)
		if err != nil {
			return err
		}
		defer cleanup()
	}

	log.Printf("All gateways ready on %s; node connection files: %s", baseURL, *stateDir)
	log.Println("Node credentials rotate on each server start. Ctrl+C to stop.")

	select {
	case <-ctx.Done():
		return nil
	case err := <-done:
		done <- err
		return fmt.Errorf("HTTP server stopped: %w", err)
	}
}

// Keep existing OAuth consent without copying or moving credential material.
// New installations use prototype-server; existing experiments retain their
// legacy state directory unless the operator explicitly selects another one.
func defaultStateDir(configDir string) string {
	current := filepath.Join(configDir, "alloy", "prototype-server")

	legacy := filepath.Join(configDir, "alloy", "prototype-notion")
	if _, err := os.Stat(current); errors.Is(err, os.ErrNotExist) {
		if info, err := os.Stat(legacy); err == nil && info.IsDir() {
			return legacy
		}
	}

	return current
}

func acquireStateLock(stateDir string) (*flock.Flock, error) {
	lock := flock.New(filepath.Join(stateDir, "server.lock"), flock.SetPermissions(0600))

	locked, err := lock.TryLock()
	if err != nil || !locked {
		_ = lock.Close()

		if err != nil {
			return nil, errors.New("cannot acquire the server state directory lock")
		}

		return nil, errors.New("another server owns this state directory")
	}

	return lock, nil
}
