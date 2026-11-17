package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	alloy "alloy/engine"
)

// notionWebhookNode is a local stand-in for the Notion webhook trigger in the
// original Nottager flow. It accepts a small test event and publishes it into
// the typed event router.
type NotionWebhookNode struct {
	mux    *http.ServeMux
	logger *log.Logger
	events chan pageEvent
}

type pageEvent struct {
	PageID  string `json:"page_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (n *NotionWebhookNode) Id() string        { return "notion-webhook" }
func (n *NotionWebhookNode) NumInstances() int { return 1 }
func (n *NotionWebhookNode) Init(s alloy.Services) error {
	n.mux, n.logger = s.HttpServerMux, s.Logger
	n.events = make(chan pageEvent, 16)
	n.mux.HandleFunc("POST /test/notion-event", n.handle)
	return nil
}
func (n *NotionWebhookNode) handle(w http.ResponseWriter, r *http.Request) {
	var event pageEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid JSON event", http.StatusBadRequest)
		return
	}
	if event.PageID == "" {
		http.Error(w, "page_id is required", http.StatusBadRequest)
		return
	}
	select {
	case <-r.Context().Done():
		return
	default:
	}
	// HTTP handlers cannot write directly to a node worker's output channel;
	// the node's Start method owns that channel, so enqueue the event there.
	select {
	case n.events <- event:
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprintln(w, `{"accepted":true}`)
	default:
		http.Error(w, "event queue full", http.StatusServiceUnavailable)
	}
}

func (n *NotionWebhookNode) Start(ctx context.Context, workerID string, _ <-chan alloy.Job, out chan<- alloy.Job) {
	n.logger.Printf("starting %s", workerID)
	for {
		select {
		case event := <-n.events:
			payload, err := json.Marshal(event)
			if err != nil {
				n.logger.Printf("encode test event: %v", err)
				continue
			}
			select {
			case out <- alloy.Job{Type: "notion.page.updated", Source: n.Id(), Payload: payload}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
