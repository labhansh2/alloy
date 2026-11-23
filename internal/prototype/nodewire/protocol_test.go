package nodewire

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http/httptest"
	"testing"
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

	server := httptest.NewServer(HTTPHandler(s, "test-token", log.New(io.Discard, "", 0)))
	defer server.Close()

	ctx := context.Background()

	endpoint := "ws" + server.URL[len("http"):] + "/node"
	for _, token := range []string{"", "wrong"} {
		if c, err := Dial(ctx, endpoint, "test-node", token); err == nil {
			c.Close()
			t.Fatal("invalid node credential was accepted")
		}
	}

	client, err := Dial(ctx, endpoint, "test-node", "test-token")
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
