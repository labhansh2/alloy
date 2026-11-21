package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"alloy/internal/prototype/nodewire"
	"alloy/internal/prototype/resources"
	"github.com/coder/websocket"
)

// NewHandler wires the prototype's loopback HTTP and node session routes.
func NewHandler(logger *log.Logger) http.Handler {
	if logger == nil {
		logger = log.Default()
	}
	protocol := &nodewire.Server{
		Resources: resources.PrototypeCLI(),
		Authorize: func(node, resource string) bool {
			return node == "prototype-node" && (resource == "echo" || resource == "ls")
		},
		OnPublish: func(_ context.Context, node string, payload json.RawMessage) error {
			logger.Printf("event from node %q: %s", node, payload)
			return nil
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /node", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		if err := protocol.ServeNode(r.Context(), conn); err != nil {
			logger.Printf("node session ended: %v", err)
		}
	})
	return mux
}

func ListenAddress() string {
	if addr := os.Getenv("ALLOY_PROTOTYPE_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:8080"
}
