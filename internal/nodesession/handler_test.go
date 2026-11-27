package nodesession

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestRejectsInvalidHandshakes(t *testing.T) {
	for _, hello := range []Message{
		{Type: "hello", Version: Version, Name: ""},
		{Type: "hello", Version: Version + 1, Name: "node"},
		{Type: "unknown", Version: Version, Name: "node"},
		{Type: "hello", Version: Version, Name: "bad name"},
	} {
		h := NewHandler(context.Background())
		srv := httptest.NewServer(h)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		conn, _, err := websocket.Dial(ctx, srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}

		if err := wsjson.Write(ctx, conn, hello); err != nil {
			t.Fatal(err)
		}

		var reply Message

		err = wsjson.Read(ctx, conn, &reply)
		_ = conn.CloseNow()

		cancel()
		h.Close()
		srv.Close()

		if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
			t.Errorf("invalid hello received %v", err)
		}
	}
}

func TestRejectsBrowserOrigin(t *testing.T) {
	h := NewHandler(context.Background())
	defer h.Close()

	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, srv.URL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{srv.URL}},
	})
	if conn != nil {
		_ = conn.CloseNow()
	}

	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("browser origin accepted: %v", err)
	}
}

func TestCloseDisconnectsIncompleteHandshake(t *testing.T) {
	h := NewHandler(context.Background())

	srv := httptest.NewServer(h)
	defer srv.Close()
	defer h.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	h.Close()

	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("session remained open after shutdown")
	}
}

func TestRejectsOversizedHandshake(t *testing.T) {
	h := NewHandler(context.Background())

	srv := httptest.NewServer(h)
	defer srv.Close()
	defer h.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	_ = conn.Write(ctx, websocket.MessageText, []byte(strings.Repeat("x", MaxBytes+1)))

	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("oversized handshake accepted")
	}
}
