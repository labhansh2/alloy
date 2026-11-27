package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestNewDoesNotStartServer(t *testing.T) {
	srv := New()
	if addr := srv.Addr(); addr != "" {
		t.Fatalf("new server has an address: %q", addr)
	}

	select {
	case <-srv.Ready():
		t.Fatal("new server is already ready")
	default:
	}
}

func TestRunServesLocallyAndStops(t *testing.T) {
	srv := New()
	startServer(t, srv)

	host, _, err := net.SplitHostPort(srv.Addr())
	if err != nil || host != "127.0.0.1" {
		t.Fatalf("server is not loopback-only: %q, %v", srv.Addr(), err)
	}

	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}

	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{path: "/health", status: http.StatusOK, body: "ok\n"},
		{path: "/", status: http.StatusNotFound, body: "404 page not found\n"},
		{path: "/node", status: http.StatusNotFound, body: "404 page not found\n"},
	} {
		resp, err := client.Get("http://" + srv.Addr() + tc.path)
		if err != nil {
			t.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != tc.status || string(body) != tc.body {
			t.Fatalf("GET %s: status=%d body=%q", tc.path, resp.StatusCode, body)
		}
	}

	if err := srv.Run(context.Background()); !errors.Is(err, ErrAlreadyRun) {
		t.Fatalf("second Run: %v", err)
	}
}

func TestInstancesHaveIndependentListeners(t *testing.T) {
	first, second := New(), New()
	startServer(t, first)
	startServer(t, second)

	if first.Addr() == second.Addr() {
		t.Fatal("instances share a listener")
	}
}

func TestCanceledContextDoesNotStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := New()
	if err := srv.Run(ctx); err != nil {
		t.Fatal(err)
	}

	if srv.Addr() != "" {
		t.Fatal("canceled server opened a listener")
	}

	select {
	case <-srv.Ready():
		t.Fatal("canceled server reported readiness")
	default:
	}

	if err := srv.Run(context.Background()); !errors.Is(err, ErrAlreadyRun) {
		t.Fatalf("restart of consumed instance: %v", err)
	}
}

func TestConcurrentRunStartsOnlyOnce(t *testing.T) {
	srv := New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := make(chan error, 2)

	for range 2 {
		go func() {
			results <- srv.Run(ctx)
		}()
	}

	defer func() {
		cancel()

		select {
		case err := <-results:
			if err != nil {
				t.Errorf("running instance did not stop normally: %v", err)
			}
		case <-time.After(7 * time.Second):
			t.Error("running instance did not stop")
		}
	}()

	select {
	case err := <-results:
		if !errors.Is(err, ErrAlreadyRun) {
			t.Fatalf("competing Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("competing Run did not return")
	}
}

func TestServingFailureReachesCaller(t *testing.T) {
	want := errors.New("listener failed")
	listener := &failingListener{err: want}

	if err := New().serve(context.Background(), listener); !errors.Is(err, want) {
		t.Fatalf("serving failure = %v; want %v", err, want)
	}

	if !listener.closed {
		t.Fatal("failed listener was not closed")
	}
}

func startServer(t *testing.T, srv *Server) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- srv.Run(ctx)
	}()

	t.Cleanup(func() {
		cancel()

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run: %v", err)
			}
		case <-time.After(7 * time.Second):
			t.Error("Run did not stop after cancellation")
		}

		if addr := srv.Addr(); addr != "" {
			conn, err := net.DialTimeout("tcp", addr, time.Second)
			if err == nil {
				_ = conn.Close()

				t.Error("listener still accepts connections after Run returned")
			}
		}
	})

	select {
	case <-srv.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("server did not become ready")
	}
}

type failingListener struct {
	err    error
	closed bool
}

func (l *failingListener) Accept() (net.Conn, error) {
	return nil, l.err
}

func (l *failingListener) Close() error {
	l.closed = true
	return nil
}

func (l *failingListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}
}
