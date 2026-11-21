// Command prototype-server runs the local process-node communication proof.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"alloy/nodewire"
	"github.com/coder/websocket"
)

func main() {
	protocol := &nodewire.Server{
		Resources: map[string]nodewire.ResourceHandler{
			"echo": func(_ context.Context, input json.RawMessage) (any, error) {
				var value any
				if err := json.Unmarshal(input, &value); err != nil {
					return nil, err
				}
				return map[string]any{"echo": value}, nil
			},
		},
		OnPublish: func(_ context.Context, node string, payload json.RawMessage) error {
			log.Printf("event from node %q: %s", node, payload)
			return nil
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /node", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		if err := protocol.ServeNode(r.Context(), conn); err != nil {
			log.Printf("node session ended: %v", err)
		}
	})
	log.Println("prototype server listening on http://127.0.0.1:8080")
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", mux))
}
