package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"alloy/internal/prototype/events"
	"alloy/internal/prototype/mcpoauth"
	"alloy/internal/prototype/nodewire"
)

func registerEvents(mux *http.ServeMux, baseURL, stateDir string) error {
	broker := events.NewBroker(16)

	for _, role := range []string{"emitter", "worker"} {
		nodeID := "prototype-event-" + role
		token := rand.Text()

		protocol := &nodewire.Server{NodeID: nodeID, Events: broker, AuthorizeEvent: eventGrant}
		if role == "worker" {
			protocol.Resources = map[string]nodewire.ResourceHandler{"echo": echo}
			protocol.Authorize = func(node, resource string) bool { return node == "prototype-event-worker" && resource == "echo" }
		}

		endpoint := "/events/" + role
		mux.Handle("GET "+endpoint, nodewire.HTTPHandler(protocol, token, log.Default()))

		path := filepath.Join(stateDir, role+"-node-connection.json")
		if err := mcpoauth.SavePrivateJSON(path, struct {
			Endpoint string `json:"endpoint"`
			Token    string `json:"token"`
		}{"ws" + baseURL[len("http"):] + endpoint, token}); err != nil {
			return err
		}

		log.Printf("Event %s connection: %s", role, path)
	}

	triggerToken := rand.Text()
	mux.Handle("POST /hooks/demo", demoTrigger(broker, triggerToken))

	path := filepath.Join(stateDir, "trigger-connection.json")
	if err := mcpoauth.SavePrivateJSON(path, struct {
		Endpoint string `json:"endpoint"`
		Token    string `json:"token"`
	}{baseURL + "/hooks/demo", triggerToken}); err != nil {
		return err
	}

	log.Printf("HTTP event trigger connection: %s", path)

	return nil
}

func eventGrant(node, action, subject string) bool {
	switch node {
	case "prototype-event-emitter":
		return (action == "publish" && subject == "demo.requested") || (action == "subscribe" && subject == "demo.completed")
	case "prototype-event-worker":
		return (action == "subscribe" && subject == "demo.requested") || (action == "publish" && subject == "demo.completed")
	default:
		return false
	}
}

// The trigger is an application endpoint, not a generic anonymous publisher or
// a third-party webhook signature implementation. It requires its own secret.
func demoTrigger(broker *events.Broker, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		defer r.Body.Close()

		var input struct {
			Text string `json:"text"`
		}

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || strings.TrimSpace(input.Text) == "" || len(input.Text) > 4096 {
			http.Error(w, "expected one JSON object with a nonempty text of at most 4096 bytes", 400)
			return
		}

		payload, err := json.Marshal(input)
		if err != nil {
			http.Error(w, "cannot encode input", 500)
			return
		}

		receipt, err := broker.Publish(r.Context(), "prototype-http-trigger", "demo.requested", payload)
		if err != nil {
			http.Error(w, "event not accepted", 503)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")

		if receipt.Queued == 0 || receipt.Dropped > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusAccepted)
		}

		_ = json.NewEncoder(w).Encode(receipt)
	})
}
