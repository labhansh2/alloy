package nodewire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"alloy/internal/prototype/events"
	"github.com/coder/websocket"
)

type Client struct {
	conn          *websocket.Conn
	mu            sync.Mutex
	writeMu       sync.Mutex
	pending       map[string]chan Message
	subscriptions map[string]chan events.Event
	closed        chan struct{}
	seq           atomic.Uint64
}

func Dial(ctx context.Context, url, node, token string) (*Client, error) {
	headers := make(http.Header)
	if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}

	httpClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: headers, HTTPClient: httpClient})
	if err != nil {
		return nil, err
	}

	conn.SetReadLimit(maxMessageSize)

	c := &Client{conn: conn, pending: make(map[string]chan Message), subscriptions: make(map[string]chan events.Event), closed: make(chan struct{})}
	if err := c.send(ctx, Message{Type: "hello", Node: node}); err != nil {
		_ = conn.CloseNow()
		return nil, err
	}

	_, data, err := conn.Read(ctx)
	if err != nil {
		_ = conn.CloseNow()
		return nil, err
	}

	var ready Message
	if json.Unmarshal(data, &ready) != nil || ready.Type != "ready" {
		_ = conn.CloseNow()
		return nil, errors.New("server did not accept node connection")
	}

	go c.readLoop()

	return c, nil
}

// Subscribe installs the local buffer before requesting server admission, so an
// event arriving before the subscribe response cannot race local registration.
// This slice has no unsubscribe operation; Close removes all subscriptions.
func (c *Client) Subscribe(ctx context.Context, subject string) (*Subscription, error) {
	c.mu.Lock()

	if c.subscriptions[subject] != nil {
		c.mu.Unlock()
		return nil, errors.New("already subscribed")
	}

	queue := make(chan events.Event, clientEventBuffer)
	c.subscriptions[subject] = queue
	c.mu.Unlock()

	if _, err := c.request(ctx, Message{Type: "subscribe", Subject: subject}, "subscribed"); err != nil {
		// Cancellation can race server admission. Without unsubscribe, closing
		// the session is the only way to remove an uncertain subscription.
		if ctx.Err() != nil {
			_ = c.conn.CloseNow()
			<-c.closed
		}

		c.mu.Lock()
		delete(c.subscriptions, subject)
		c.mu.Unlock()

		return nil, err
	}

	return &Subscription{queue: queue, closed: c.closed}, nil
}

type Subscription struct {
	queue  <-chan events.Event
	closed <-chan struct{}
}

func (s *Subscription) Next(ctx context.Context) (events.Event, error) {
	select {
	case <-ctx.Done():
		return events.Event{}, ctx.Err()
	case <-s.closed:
		return events.Event{}, errors.New("node connection closed")
	case event := <-s.queue:
		return event, nil
	}
}

func (c *Client) Emit(ctx context.Context, subject string, payload any) (events.Receipt, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return events.Receipt{}, err
	}

	result, err := c.request(ctx, Message{Type: "emit", Subject: subject, Payload: data}, "emitted")
	if err != nil {
		return events.Receipt{}, err
	}

	if result.Receipt == nil {
		return events.Receipt{}, errors.New("missing emission receipt")
	}

	return *result.Receipt, nil
}

// Publish preserves the original untyped notification demo. New event routing
// uses Emit with an explicit subject and receipt.
func (c *Client) Publish(ctx context.Context, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = c.request(ctx, Message{Type: "publish", Payload: data}, "published")

	return err
}

func (c *Client) CallResource(ctx context.Context, name string, input, output any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}

	result, err := c.request(ctx, Message{Type: "resource.call", Name: name, Payload: data}, "resource.result")
	if err != nil {
		return err
	}

	if output == nil {
		return nil
	}

	return json.Unmarshal(result.Payload, output)
}

func (c *Client) request(ctx context.Context, msg Message, expected string) (Message, error) {
	msg.ID = fmt.Sprintf("%d", c.seq.Add(1))
	ch := make(chan Message, 1)

	c.mu.Lock()
	c.pending[msg.ID] = ch
	c.mu.Unlock()

	defer func() { c.mu.Lock(); delete(c.pending, msg.ID); c.mu.Unlock() }()

	if err := c.send(ctx, msg); err != nil {
		return Message{}, err
	}

	select {
	case result := <-ch:
		if result.Type == "error" {
			return Message{}, errors.New(result.Error)
		}

		if result.Type != expected {
			return Message{}, fmt.Errorf("unexpected response %q", result.Type)
		}

		return result, nil
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case <-c.closed:
		return Message{}, errors.New("node connection closed")
	}
}

func (c *Client) send(ctx context.Context, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.conn.Write(ctx, websocket.MessageText, data)
}

func (c *Client) readLoop() {
	defer close(c.closed)
	defer c.conn.CloseNow()

	for {
		_, data, err := c.conn.Read(context.Background())
		if err != nil {
			return
		}

		var msg Message
		if json.Unmarshal(data, &msg) != nil {
			return
		}

		if msg.Type == "event" {
			if msg.Event == nil {
				return
			}

			c.mu.Lock()
			queue := c.subscriptions[msg.Event.Subject]
			c.mu.Unlock()

			if queue == nil {
				return
			}

			select {
			case queue <- *msg.Event:
			default:
				return
			}

			continue
		}

		c.mu.Lock()
		ch := c.pending[msg.ID]
		c.mu.Unlock()

		if ch != nil {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

func (c *Client) Close() error {
	err := c.conn.Close(websocket.StatusNormalClosure, "node stopped")
	_ = c.conn.CloseNow()
	<-c.closed

	return err
}
