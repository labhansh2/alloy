package node_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"alloy/node"
	"alloy/server"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestConfigValidation(t *testing.T) {
	for _, config := range []node.Config{
		{ServerURL: "http://127.0.0.1:8080"},
		{Name: "bad name", ServerURL: "http://127.0.0.1:8080"},
		{Name: strings.Repeat("a", 129), ServerURL: "http://127.0.0.1:8080"},
		{Name: "node"},
		{Name: "node", ServerURL: "http://example.com:8080"},
		{Name: "node", ServerURL: "http://localhost:8080"},
		{Name: "node", ServerURL: "http://0.0.0.0:8080"},
		{Name: "node", ServerURL: "http://127.0.0.1"},
		{Name: "node", ServerURL: "http://127.0.0.1:0"},
		{Name: "node", ServerURL: "http://127.0.0.1:65536"},
		{Name: "node", ServerURL: "https://127.0.0.1:8080"},
		{Name: "node", ServerURL: "http://user:secret@127.0.0.1:8080"},
		{Name: "node", ServerURL: "http://127.0.0.1:8080/path"},
		{Name: "node", ServerURL: "http://127.0.0.1:8080?token=secret"},
		{Name: "node", ServerURL: "http://127.0.0.1:8080#fragment"},
	} {
		if _, err := node.New(config); err == nil {
			t.Errorf("invalid config accepted: name=%q", config.Name)
		}
	}

	for _, address := range []string{"http://127.0.0.1:8080", "http://[::1]:8080/"} {
		if _, err := node.New(node.Config{Name: "watcher-1.test", ServerURL: address}); err != nil {
			t.Errorf("valid config rejected: %v", err)
		}
	}
}

func TestNodeConnectsAndStops(t *testing.T) {
	srv := server.New()
	start(t, srv.Run, srv.Ready())

	n := newNode(t, "http://"+srv.Addr())
	select {
	case <-n.Ready():
		t.Fatal("New started a session")
	default:
	}

	stop := start(t, n.Run, n.Ready())
	if err := n.Run(context.Background()); !errors.Is(err, node.ErrAlreadyRun) {
		t.Fatalf("concurrent Run: %v", err)
	}

	stop()

	if err := n.Run(context.Background()); !errors.Is(err, node.ErrAlreadyRun) {
		t.Fatalf("restart: %v", err)
	}
}

func TestServerShutdownDisconnectsNode(t *testing.T) {
	srv := server.New()
	stopServer := start(t, srv.Run, srv.Ready())
	n := newNode(t, "http://"+srv.Addr())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- n.Run(ctx)
	}()

	select {
	case <-n.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("node did not connect")
	}

	stopServer()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("server shutdown was reported as a successful node run")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("node did not detect server shutdown")
	}
}

func TestCanceledContextDoesNotConnect(t *testing.T) {
	n := newNode(t, "http://127.0.0.1:1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := n.Run(ctx); err != nil {
		t.Fatal(err)
	}

	assertNotReady(t, n)
}

func TestUnreachableServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	address := "http://" + listener.Addr().String()
	_ = listener.Close()

	n := newNode(t, address)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := n.Run(ctx); err == nil {
		t.Fatal("unreachable server accepted")
	}

	assertNotReady(t, n)
}

func TestInvalidHandshake(t *testing.T) {
	for _, reply := range []string{
		`{"type":"welcome","version":999}`,
		`{"type":"unknown","version":1}`,
		`not JSON`,
	} {
		t.Run(reply, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()

				ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
				defer cancel()

				var hello map[string]any
				if wsjson.Read(ctx, conn, &hello) == nil {
					_ = conn.Write(ctx, websocket.MessageText, []byte(reply))
				}
			}))
			defer srv.Close()

			n := newNode(t, srv.URL)
			if err := n.Run(context.Background()); err == nil {
				t.Fatal("invalid handshake accepted")
			}

			assertNotReady(t, n)
		})
	}
}

func TestRejectsHTTPAndRedirects(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusTemporaryRedirect} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "http://example.com")
			w.WriteHeader(status)
		}))

		n := newNode(t, srv.URL)
		err := n.Run(context.Background())

		srv.Close()

		if err == nil {
			t.Errorf("HTTP status %d accepted as a node session", status)
		}

		assertNotReady(t, n)
	}
}

func TestCancellationDuringHandshake(t *testing.T) {
	accepted := make(chan struct{})
	closed := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(closed)

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		var hello map[string]any
		if wsjson.Read(ctx, conn, &hello) != nil {
			return
		}

		close(accepted)
		// Deliberately withhold the welcome message.
		_, _, _ = conn.Read(ctx)
	}))
	defer srv.Close()

	n := newNode(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- n.Run(ctx)
	}()

	select {
	case <-accepted:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not receive handshake")
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("node did not stop during handshake")
	}

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("canceled handshake left a connection alive")
	}

	assertNotReady(t, n)
}

func newNode(t *testing.T, address string) *node.Node {
	t.Helper()

	n, err := node.New(node.Config{Name: "test-node", ServerURL: address})
	if err != nil {
		t.Fatal(err)
	}

	return n
}

func assertNotReady(t *testing.T, n *node.Node) {
	t.Helper()

	select {
	case <-n.Ready():
		t.Error("unsuccessful handshake reported readiness")
	default:
	}
}

func start(t *testing.T, run func(context.Context) error, ready <-chan struct{}) func() {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	var runErr error

	go func() {
		defer close(done)

		runErr = run(ctx)
	}()

	var once sync.Once

	stop := func() {
		once.Do(func() {
			cancel()

			select {
			case <-done:
				if runErr != nil {
					t.Errorf("Run: %v", runErr)
				}
			case <-time.After(7 * time.Second):
				t.Error("Run did not stop")
			}
		})
	}
	t.Cleanup(stop)

	select {
	case <-ready:
	case <-done:
		t.Fatalf("Run failed before readiness: %v", runErr)
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not become ready")
	}

	return stop
}
