package nodewire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"alloy/internal/prototype/events"
	"github.com/coder/websocket"
)

type Server struct {
	// NodeID is the identity assigned to this authenticated endpoint. A caller
	// cannot obtain another identity merely by changing its hello message.
	NodeID         string
	Resources      map[string]ResourceHandler
	OnPublish      func(context.Context, string, json.RawMessage) error // legacy untyped notification
	Authorize      func(node, resource string) bool
	Events         *events.Broker
	AuthorizeEvent func(node, action, subject string) bool
}

func (s *Server) ServeNode(ctx context.Context, conn *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn.SetReadLimit(maxMessageSize)

	_, data, err := conn.Read(ctx)
	if err != nil {
		return err
	}

	var hello Message
	if json.Unmarshal(data, &hello) != nil || hello.Type != "hello" || s.NodeID == "" || hello.Node != s.NodeID {
		return errors.New("node identity does not match its authenticated endpoint")
	}

	var writeMu sync.Mutex

	write := func(msg Message) error {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}

		writeMu.Lock()
		defer writeMu.Unlock()

		writeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()

		return conn.Write(writeCtx, websocket.MessageText, data)
	}
	if err := write(Message{Type: "ready"}); err != nil {
		return err
	}

	subscriptions := make(map[string]*events.Subscription)

	var forwards sync.WaitGroup

	defer func() {
		cancel()

		for _, sub := range subscriptions {
			sub.Close()
		}

		forwards.Wait()
	}()

	subscribe := func(subject string) error {
		if subscriptions[subject] != nil {
			return nil
		}

		sub, err := s.Events.Subscribe(s.NodeID, subject)
		if err != nil {
			return err
		}

		subscriptions[subject] = sub

		forwards.Go(func() {
			for {
				event, err := sub.Next(ctx)
				if err != nil {
					return
				}

				if err := write(Message{Type: "event", Event: &event}); err != nil {
					cancel()

					_ = conn.CloseNow()

					return
				}
			}
		})

		return nil
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}

		reply := s.handle(ctx, msg, subscribe)

		reply.ID = msg.ID
		if err := write(reply); err != nil {
			return err
		}
	}
}

func (s *Server) handle(ctx context.Context, msg Message, subscribe func(string) error) Message {
	failure := func(err error) Message { return Message{Type: "error", Error: err.Error()} }

	switch msg.Type {
	case "subscribe", "emit":
		action := "subscribe"
		if msg.Type == "emit" {
			action = "publish"
		}

		if s.Events == nil || s.AuthorizeEvent == nil || !s.AuthorizeEvent(s.NodeID, action, msg.Subject) {
			return failure(errors.New("event operation not allowed"))
		}

		if msg.Type == "subscribe" {
			if err := subscribe(msg.Subject); err != nil {
				return failure(err)
			}

			return Message{Type: "subscribed"}
		}

		receipt, err := s.Events.Publish(ctx, s.NodeID, msg.Subject, msg.Payload)
		if err != nil {
			return failure(err)
		}

		return Message{Type: "emitted", Receipt: &receipt}
	case "publish":
		if s.OnPublish != nil {
			if err := s.OnPublish(ctx, s.NodeID, msg.Payload); err != nil {
				return failure(err)
			}
		}

		return Message{Type: "published"}
	case "resource.call":
		handler := s.Resources[msg.Name]
		if handler == nil || (s.Authorize != nil && !s.Authorize(s.NodeID, msg.Name)) {
			return failure(errors.New("resource not allowed: " + msg.Name))
		}

		result, err := handler(ctx, msg.Payload)
		if err != nil {
			return failure(err)
		}

		payload, err := json.Marshal(result)
		if err != nil {
			return failure(err)
		}

		return Message{Type: "resource.result", Payload: payload}
	default:
		return failure(fmt.Errorf("unknown message type %q", msg.Type))
	}
}
