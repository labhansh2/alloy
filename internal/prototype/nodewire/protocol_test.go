package nodewire

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
)

func TestNodeSessionPublishesAndCallsApprovedResource(t *testing.T) {
	s := &Server{Resources: map[string]ResourceHandler{"echo": func(_ context.Context, in json.RawMessage) (any, error) {
		var v any
		if err := json.Unmarshal(in, &v); err != nil {
			return nil, err
		}
		return map[string]any{"echo": v}, nil
	}}, Authorize: func(node, resource string) bool { return node == "test-node" && resource == "echo" }, OnPublish: func(_ context.Context, node string, _ json.RawMessage) error {
		if node != "test-node" {
			t.Errorf("publish node = %q", node)
		}
		return nil
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /node", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		_ = s.ServeNode(r.Context(), conn)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx := context.Background()
	client, err := Dial(ctx, "ws"+server.URL[len("http"):]+"/node", "test-node")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var got map[string]any
	if err := client.CallResource(ctx, "echo", map[string]string{"key": "value"}, &got); err != nil {
		t.Fatal(err)
	}
	if got["echo"] == nil {
		t.Fatalf("unexpected resource response: %#v", got)
	}
	if err := client.Publish(ctx, map[string]string{"event": "test"}); err != nil {
		t.Fatal(err)
	}
	if err := client.CallResource(ctx, "not-approved", nil, &got); err == nil {
		t.Fatal("expected unapproved resource to be denied")
	}
}
