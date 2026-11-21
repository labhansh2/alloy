// Package nodewire contains the deliberately small WebSocket protocol used by
// the process-node prototype. It is an internal wire contract, not a user SDK.
package nodewire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const maxMessageSize = 1 << 20

type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Node    string          `json:"node,omitempty"`
	Name    string          `json:"name,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// ResourceHandler is server-owned: nodes request an operation, but never
// receive the server credential or implementation behind that resource.
type ResourceHandler func(context.Context, json.RawMessage) (any, error)

type Server struct {
	Resources map[string]ResourceHandler
	OnPublish func(context.Context, string, json.RawMessage) error
}

func (s *Server) ServeNode(ctx context.Context, conn *websocket.Conn) error {
	conn.SetReadLimit(maxMessageSize)
	_, data, err := conn.Read(ctx)
	if err != nil {
		return err
	}
	var hello Message
	if err := json.Unmarshal(data, &hello); err != nil || hello.Type != "hello" || hello.Node == "" {
		return errors.New("first node message must be hello with a node name")
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"ready"}`)); err != nil {
		return err
	}

	writeMu := &sync.Mutex{}
	write := func(m Message) error {
		data, err := json.Marshal(m)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.Write(ctx, websocket.MessageText, data)
	}
	for {
		_, data, err = conn.Read(ctx)
		if err != nil {
			return err
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		switch msg.Type {
		case "publish":
			if s.OnPublish != nil {
				if err := s.OnPublish(ctx, hello.Node, msg.Payload); err != nil {
					if err := write(Message{Type: "error", ID: msg.ID, Error: err.Error()}); err != nil {
						return err
					}
					continue
				}
			}
			if err := write(Message{Type: "published", ID: msg.ID}); err != nil {
				return err
			}
		case "resource.call":
			handler := s.Resources[msg.Name]
			if handler == nil {
				if err := write(Message{Type: "error", ID: msg.ID, Error: "resource not allowed: " + msg.Name}); err != nil {
					return err
				}
				continue
			}
			result, err := handler(ctx, msg.Payload)
			if err != nil {
				if err := write(Message{Type: "error", ID: msg.ID, Error: err.Error()}); err != nil {
					return err
				}
				continue
			}
			payload, err := json.Marshal(result)
			if err != nil {
				return err
			}
			if err := write(Message{Type: "resource.result", ID: msg.ID, Payload: payload}); err != nil {
				return err
			}
		default:
			if err := write(Message{Type: "error", ID: msg.ID, Error: fmt.Sprintf("unknown message type %q", msg.Type)}); err != nil {
				return err
			}
		}
	}
}

type Client struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	writeMu sync.Mutex
	pending map[string]chan Message
	closed  chan struct{}
	seq     atomic.Uint64
}

func Dial(ctx context.Context, url, node string) (*Client, error) {
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(maxMessageSize)
	c := &Client{conn: conn, pending: make(map[string]chan Message), closed: make(chan struct{})}
	if err := c.send(ctx, Message{Type: "hello", Node: node}); err != nil {
		conn.Close(websocket.StatusInternalError, "hello failed")
		return nil, err
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "handshake failed")
		return nil, err
	}
	var ready Message
	if err := json.Unmarshal(data, &ready); err != nil || ready.Type != "ready" {
		conn.Close(websocket.StatusPolicyViolation, "invalid handshake")
		return nil, errors.New("server did not accept node connection")
	}
	go c.readLoop()
	return c, nil
}

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
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		_, data, err := c.conn.Read(ctx)
		cancel()
		if err != nil {
			return
		}
		var msg Message
		if json.Unmarshal(data, &msg) != nil {
			return
		}
		c.mu.Lock()
		ch := c.pending[msg.ID]
		c.mu.Unlock()
		if ch != nil {
			ch <- msg
		}
	}
}

func (c *Client) Close() error {
	err := c.conn.Close(websocket.StatusNormalClosure, "node stopped")
	select {
	case <-c.closed:
	default:
	}
	return err
}
