// Package events is a bounded, ephemeral broker. It provides no persistence,
// acknowledgements, replay, retries, or competing replicas.
package events

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

type Event struct {
	ID      string          `json:"id"`
	Subject string          `json:"subject"`
	Source  string          `json:"source"`
	Payload json.RawMessage `json:"payload"`
}

// Leave room for metadata under the transport's 1 MiB frame cap.
const MaxPayloadSize = 512 << 10

// Receipt reports admission to server-side subscriber queues, not processing.
// An accepted event may still be lost on disconnect or process failure.
type Receipt struct {
	EventID string `json:"eventId"`
	Queued  int    `json:"queued"`
	Dropped int    `json:"dropped"`
}

type Broker struct {
	mu        sync.Mutex
	queueSize int
	subjects  map[string]map[string]*Subscription
}

func NewBroker(queueSize int) *Broker {
	if queueSize < 1 {
		panic("event queue size must be positive")
	}

	return &Broker{queueSize: queueSize, subjects: make(map[string]map[string]*Subscription)}
}

// Subscribe allows one active subscriber per logical node/subject. Distinct
// logical subscribers receive independent copies; replicas are not implemented.
func (b *Broker) Subscribe(node, subject string) (*Subscription, error) {
	if node == "" || !validSubject(subject) {
		return nil, errors.New("node and exact subject are required")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subjects[subject] == nil {
		b.subjects[subject] = make(map[string]*Subscription)
	}

	if b.subjects[subject][node] != nil {
		return nil, errors.New("node already has an active subscription to this subject")
	}

	s := &Subscription{broker: b, node: node, subject: subject, queue: make(chan Event, b.queueSize)}
	b.subjects[subject][node] = s

	return s, nil
}

// Publish never waits on a slow subscriber. Full queues drop that subscriber's
// copy and expose the count to the caller. Payloads must be bounded valid JSON.
func (b *Broker) Publish(ctx context.Context, source, subject string, payload json.RawMessage) (Receipt, error) {
	if source == "" || len(source) > 128 || !validSubject(subject) || len(payload) > MaxPayloadSize || !json.Valid(payload) {
		return Receipt{}, errors.New("source, exact subject and JSON payload of at most 512 KiB are required")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}

	event := Event{ID: rand.Text(), Subject: subject, Source: source}

	receipt := Receipt{EventID: event.ID}
	for _, s := range b.subjects[subject] {
		copy := event

		copy.Payload = append(json.RawMessage(nil), payload...)
		select {
		case s.queue <- copy:
			receipt.Queued++
		default:
			receipt.Dropped++
		}
	}

	return receipt, nil
}

func validSubject(subject string) bool {
	return subject != "" && len(subject) <= 128 && !strings.ContainsAny(subject, " \t\r\n*>")
}

type Subscription struct {
	broker        *Broker
	node, subject string
	queue         chan Event
	closed        bool // guarded by broker.mu, as are queue sends and closure
}

func (s *Subscription) Next(ctx context.Context) (Event, error) {
	select {
	case <-ctx.Done():
		return Event{}, ctx.Err()
	case event, ok := <-s.queue:
		if !ok {
			return Event{}, errors.New("subscription closed")
		}

		return event, nil
	}
}

func (s *Subscription) Close() {
	s.broker.mu.Lock()
	defer s.broker.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	delete(s.broker.subjects[s.subject], s.node)

	if len(s.broker.subjects[s.subject]) == 0 {
		delete(s.broker.subjects, s.subject)
	}

	close(s.queue)
}
