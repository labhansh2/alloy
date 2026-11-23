package nodewire

import (
	"crypto/subtle"
	"log"
	"net/http"

	"github.com/coder/websocket"
)

// HTTPHandler upgrades authenticated requests to the node protocol. The
// application owns route placement, resource registration, and node policy.
func HTTPHandler(protocol *Server, token string, logger *log.Logger) http.Handler {
	if logger == nil {
		logger = log.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		if err := protocol.ServeNode(r.Context(), conn); err != nil {
			logger.Printf("node session ended: %v", err)
		}
	})
}
