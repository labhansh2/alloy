package events

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestFanoutCopiesAndSubscriptionLifetime(t *testing.T) {
	b := NewBroker(1)

	first, err := b.Subscribe("first", "test.event")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	second, err := b.Subscribe("second", "test.event")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	if _, err := b.Subscribe("first", "test.event"); err == nil {
		t.Fatal("duplicate logical subscriber accepted")
	}

	payload := json.RawMessage(`{"value":1}`)

	receipt, err := b.Publish(context.Background(), "producer", "test.event", payload)
	if err != nil || receipt.Queued != 2 || receipt.Dropped != 0 {
		t.Fatalf("bad receipt: %+v %v", receipt, err)
	}

	payload[0] = 'x'

	a, err := first.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	a.Payload[0] = 'y'

	c, err := second.Next(context.Background())
	if err != nil || !json.Valid(c.Payload) || c.ID != receipt.EventID || c.Source != "producer" {
		t.Fatalf("fanout not isolated: %+v %v", c, err)
	}

	first.Close()
	first.Close()

	replacement, err := b.Subscribe("first", "test.event")
	if err != nil {
		t.Fatal("closed subscription was not removed:", err)
	}

	replacement.Close()
}

func TestSlowSubscriberDoesNotBlockOthers(t *testing.T) {
	b := NewBroker(1)

	slow, err := b.Subscribe("slow", "test.event")
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Close()

	fast, err := b.Subscribe("fast", "test.event")
	if err != nil {
		t.Fatal(err)
	}
	defer fast.Close()

	if _, err := b.Publish(context.Background(), "producer", "test.event", json.RawMessage(`1`)); err != nil {
		t.Fatal(err)
	}

	if _, err := fast.Next(context.Background()); err != nil {
		t.Fatal(err)
	}

	receipt, err := b.Publish(context.Background(), "producer", "test.event", json.RawMessage(`2`))
	if err != nil || receipt.Queued != 1 || receipt.Dropped != 1 {
		t.Fatalf("slow queue not reported: %+v %v", receipt, err)
	}

	event, err := fast.Next(context.Background())
	if err != nil || string(event.Payload) != "2" {
		t.Fatalf("fast subscriber was blocked: %+v %v", event, err)
	}

	empty, err := b.Publish(context.Background(), "producer", "no.subscribers", json.RawMessage(`null`))
	if err != nil || empty.Queued != 0 || empty.Dropped != 0 {
		t.Fatalf("no-subscriber receipt: %+v %v", empty, err)
	}
}

func TestRejectsInvalidEventsAndCancellation(t *testing.T) {
	b := NewBroker(1)
	for _, subject := range []string{"", "with spaces", "wild.*", strings.Repeat("a", 129)} {
		if _, err := b.Subscribe("node", subject); err == nil {
			t.Errorf("accepted subject %q", subject)
		}
	}

	for _, payload := range []json.RawMessage{nil, json.RawMessage(`{`), json.RawMessage(`"` + strings.Repeat("x", MaxPayloadSize) + `"`)} {
		if _, err := b.Publish(context.Background(), "producer", "test.event", payload); err == nil {
			t.Fatal("invalid payload accepted")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := b.Publish(ctx, "producer", "test.event", json.RawMessage(`null`)); err == nil {
		t.Fatal("canceled publication accepted")
	}

	sub, err := b.Subscribe("node", "test.event")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	if _, err := sub.Next(ctx); err == nil {
		t.Fatal("Next did not honor cancellation")
	}
}

func TestConcurrentPublishAndClose(t *testing.T) {
	b := NewBroker(1)

	sub, err := b.Subscribe("node", "test.event")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				if _, err := b.Publish(context.Background(), "producer", "test.event", json.RawMessage(`{}`)); err != nil {
					t.Error(err)
				}
			}
		})
	}

	wg.Go(sub.Close)
	wg.Wait()
}
