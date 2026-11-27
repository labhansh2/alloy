package nodesession

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// Handler accepts local sessions, optionally authenticated before upgrading.
// Close cancels and waits for sessions, including hijacked HTTP connections.
type Handler struct {
	// Authenticate validates a supplied credential and returns a client identity.
	// Configure before serving. An absent credential remains handshake-only.
	Authenticate func(*http.Request) (string, bool)
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
	closed       bool
	wg           sync.WaitGroup
}

func NewHandler(ctx context.Context) *Handler {
	ctx, cancel := context.WithCancel(ctx)

	return &Handler{ctx: ctx, cancel: cancel}
}

func (h *Handler) Close() {
	h.mu.Lock()
	h.closed = true
	h.cancel()
	h.mu.Unlock()
	h.wg.Wait()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A local browser page must not acquire a session through ambient access.
	if r.Header.Get("Origin") != "" {
		http.Error(w, "browser node sessions are not supported", http.StatusForbidden)
		return
	}

	client := ""

	if r.Header.Get("Authorization") != "" {
		var ok bool
		if h.Authenticate != nil {
			client, ok = h.Authenticate(r)
		}

		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	h.mu.Lock()

	if h.closed {
		h.mu.Unlock()
		http.Error(w, "server is stopping", http.StatusServiceUnavailable)

		return
	}

	h.wg.Add(1)

	h.mu.Unlock()
	defer h.wg.Done()

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	conn.SetReadLimit(MaxBytes)

	handshakeCtx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
	defer cancel()

	var hello Message
	if err := wsjson.Read(handshakeCtx, conn, &hello); err != nil {
		return
	}

	if hello.Type != "hello" || hello.Version != Version || ValidateName(hello.Name) != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid node handshake")
		return
	}

	log.Printf("Alloy server handshake: received hello (name=%q version=%d client=%q)", hello.Name, hello.Version, client)

	if err := wsjson.Write(handshakeCtx, conn, Message{Type: "welcome", Version: Version, Client: client}); err != nil {
		return
	}

	log.Printf("Alloy server handshake: sent welcome (name=%q version=%d)", hello.Name, Version)

	cancel()

	// There are no application messages yet. Read handles control frames and
	// detects disconnects; cancellation closes the underlying connection.
	if _, _, err := conn.Read(h.ctx); err == nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "node messages are not supported yet")
	}
}
