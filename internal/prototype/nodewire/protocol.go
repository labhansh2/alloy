// Package nodewire contains the small WebSocket protocol used by process-node
// prototypes. It is an internal wire contract, not a public SDK.
package nodewire

import (
	"context"
	"encoding/json"

	"alloy/internal/prototype/events"
)

const maxMessageSize = 1 << 20

// Server-side and client-side event buffers are bounded independently. Exceeding
// the client buffer closes the session rather than blocking response dispatch.
const clientEventBuffer = 16

type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Node    string          `json:"node,omitempty"`
	Name    string          `json:"name,omitempty"`
	Subject string          `json:"subject,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
	Event   *events.Event   `json:"event,omitempty"`
	Receipt *events.Receipt `json:"receipt,omitempty"`
}

// ResourceHandler executes a server-owned operation without distributing its
// implementation or credentials to the caller.
type ResourceHandler func(context.Context, json.RawMessage) (any, error)
