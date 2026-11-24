package nodewire

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"alloy/internal/prototype/events"
)

func testPeer(t *testing.T, ctx context.Context, server *Server) *Client {
	t.Helper()

	host := httptest.NewServer(HTTPHandler(server, "test-token", log.New(io.Discard, "", 0)))
	t.Cleanup(host.Close)

	endpoint := "ws" + host.URL[len("http"):]
	if c, err := Dial(ctx, endpoint, "forged-node", "test-token"); err == nil {
		_ = c.Close()

		t.Fatal("endpoint accepted a forged node identity")
	}

	node, err := Dial(ctx, endpoint, server.NodeID, "test-token")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = node.Close() })

	return node
}

func TestEventRoundTripAndResourceCallsOnSameConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b := events.NewBroker(8)
	emitter := testPeer(t, ctx, &Server{NodeID: "emitter", Events: b, AuthorizeEvent: func(_, action, subject string) bool {
		return (action == "publish" && subject == "demo.requested") || (action == "subscribe" && subject == "demo.completed")
	}})
	worker := testPeer(t, ctx, &Server{NodeID: "worker", Events: b, AuthorizeEvent: func(_, action, subject string) bool {
		return (action == "subscribe" && subject == "demo.requested") || (action == "publish" && subject == "demo.completed")
	}, Resources: map[string]ResourceHandler{"echo": func(_ context.Context, in json.RawMessage) (any, error) { return map[string]any{"echo": in}, nil }}})

	completed, err := emitter.Subscribe(ctx, "demo.completed")
	if err != nil {
		t.Fatal(err)
	}

	jobs, err := worker.Subscribe(ctx, "demo.requested")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := emitter.Emit(ctx, "forbidden", nil); err == nil {
		t.Fatal("unauthorized emission accepted")
	}

	if _, err := emitter.Subscribe(ctx, "demo.requested"); err == nil {
		t.Fatal("unauthorized subscription accepted")
	}

	receipt, err := emitter.Emit(ctx, "demo.requested", map[string]string{"text": "hello"})
	if err != nil || receipt.Queued != 1 {
		t.Fatalf("emit failed: %+v %v", receipt, err)
	}

	job, err := jobs.Next(ctx)
	if err != nil || job.ID != receipt.EventID || job.Source != "emitter" {
		t.Fatalf("unexpected delivery: %+v %v", job, err)
	}

	var resource map[string]any
	if err := worker.CallResource(ctx, "echo", map[string]string{"text": "hello"}, &resource); err != nil || resource["echo"] == nil {
		t.Fatalf("resource response blocked by events: %v", err)
	}

	if _, err := worker.Emit(ctx, "demo.completed", map[string]string{"requestId": job.ID}); err != nil {
		t.Fatal(err)
	}

	completion, err := completed.Next(ctx)
	if err != nil || completion.Source != "worker" {
		t.Fatalf("completion not received: %+v %v", completion, err)
	}

	var data map[string]string
	if json.Unmarshal(completion.Payload, &data) != nil || data["requestId"] != job.ID {
		t.Fatal("completion lost correlation")
	}
}

func TestClientBackpressureClosesInsteadOfBlockingResponses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b := events.NewBroker(128)

	node := testPeer(t, ctx, &Server{NodeID: "slow", Events: b, AuthorizeEvent: func(_, action, subject string) bool { return action == "subscribe" && subject == "test.event" }})
	if _, err := node.Subscribe(ctx, "test.event"); err != nil {
		t.Fatal(err)
	}

	for range 64 {
		if _, err := b.Publish(ctx, "producer", "test.event", json.RawMessage(`{}`)); err != nil {
			t.Fatal(err)
		}
	}

	select {
	case <-node.closed:
	case <-ctx.Done():
		t.Fatal("slow client did not close at its buffer limit")
	}
}

func TestDisconnectRemovesSubscriptionsAndJoinsForwarders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b := events.NewBroker(1)
	s := &Server{NodeID: "worker", Events: b, AuthorizeEvent: func(_, _, _ string) bool { return true }}
	handler := HTTPHandler(s, "test-token", log.New(io.Discard, "", 0))
	done := make(chan struct{})

	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
		close(done)
	}))
	defer host.Close()

	node, err := Dial(ctx, "ws"+host.URL[len("http"):], "worker", "test-token")
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()

	if _, err := node.Subscribe(ctx, "test.event"); err != nil {
		t.Fatal(err)
	}

	_ = node.Close()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("server did not join event forwarders after disconnect")
	}

	replacement, err := b.Subscribe("worker", "test.event")
	if err != nil {
		t.Fatal("disconnect left a subscription registered:", err)
	}

	replacement.Close()
}

func TestCanceledSubscriptionClosesUncertainSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	node := testPeer(t, ctx, &Server{NodeID: "worker", Events: events.NewBroker(1), AuthorizeEvent: func(_, _, _ string) bool { return true }})
	canceled, stop := context.WithCancel(ctx)
	stop()

	if _, err := node.Subscribe(canceled, "test.event"); err == nil {
		t.Fatal("canceled subscription succeeded")
	}

	select {
	case <-node.closed:
	case <-ctx.Done():
		t.Fatal("uncertain subscription left its connection open")
	}
}
