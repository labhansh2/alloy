// Package server constructs and runs Alloy servers. Networking and protocol
// setup belong to the server, not to the application that starts it.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"alloy/internal/nodesession"
)

// ErrAlreadyRun means Run has already been called on this instance.
var ErrAlreadyRun = errors.New("alloy server instance has already run")

// Server is one local Alloy server instance. Construct it with New.
// Its methods are safe to call concurrently; a Server must not be copied.
// An instance can run only once, even if startup fails.
type Server struct {
	mu      sync.Mutex
	started bool
	addr    string
	ready   chan struct{}
}

// New creates an instance without opening sockets, starting goroutines, reading
// credentials, or writing files. Call Run to start it.
func New() *Server {
	return &Server{ready: make(chan struct{})}
}

// Addr returns the bound local address, or an empty string before startup.
// After shutdown it retains the last address; it is not a liveness check.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.addr
}

// Ready closes once the listener is bound and serving is starting. It stays
// closed after shutdown. If startup fails it never closes, so callers waiting
// for readiness must also observe Run's result or their own context.
func (s *Server) Ready() <-chan struct{} {
	return s.ready
}

// Run starts a loopback-only server on an available port and blocks until ctx
// is canceled or serving fails. Cancellation is a normal stop and returns nil.
// Startup and serving errors are returned to the caller. Run shuts down its
// listener and joins its serving goroutine before returning.
//
// This initial SDK serves a health check and unauthenticated local node
// sessions. It does not import host credentials or expose resources, events,
// or administrative operations. Node names are not authenticated identities.
func (s *Server) Run(ctx context.Context) error {
	s.mu.Lock()

	if s.started {
		s.mu.Unlock()
		return ErrAlreadyRun
	}

	s.started = true
	s.mu.Unlock()

	if ctx.Err() != nil {
		return nil
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}

		return fmt.Errorf("start alloy server: %w", err)
	}

	return s.serve(ctx, listener)
}

func (s *Server) serve(ctx context.Context, listener net.Listener) error {
	sessions := nodesession.NewHandler(ctx)
	defer sessions.Close()

	mux := http.NewServeMux()
	mux.Handle("GET "+nodesession.Path, sessions)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	defer httpServer.Close()

	done := make(chan error, 1)

	go func() {
		done <- httpServer.Serve(listener)
	}()

	s.mu.Lock()
	s.addr = listener.Addr().String()
	close(s.ready)
	s.mu.Unlock()

	select {
	case err := <-done:
		return fmt.Errorf("serve alloy server: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		shutdownErr := httpServer.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			// A timed-out drain must not leave the listener or connections alive.
			_ = httpServer.Close()
		}

		serveErr := <-done
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}

		if err := errors.Join(shutdownErr, serveErr); err != nil {
			return fmt.Errorf("stop alloy server: %w", err)
		}

		return nil
	}
}
