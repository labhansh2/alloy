package nodes

import (
	"context"
	"encoding/json"
	"log"

	alloy "alloy/engine"
)

// detectPageUpdate and updateTags model the two processing stages in the
// previous Nottager flow without calling Notion or an AI provider. They make
// typed routing and fan-out testable with no external credentials.
type DetectPageUpdate struct{ logger *log.Logger }

func (n *DetectPageUpdate) Id() string                  { return "detect-page-update" }
func (n *DetectPageUpdate) NumInstances() int           { return 1 }
func (n *DetectPageUpdate) Init(s alloy.Services) error { n.logger = s.Logger; return nil }
func (n *DetectPageUpdate) Start(ctx context.Context, workerID string, in <-chan alloy.Job, out chan<- alloy.Job) {
	n.logger.Printf("starting %s", workerID)
	for {
		select {
		case job, ok := <-in:
			if !ok {
				return
			}
			var event pageEvent
			if err := json.Unmarshal(job.Payload, &event); err != nil {
				n.logger.Printf("decode page event: %v", err)
				continue
			}
			payload, err := json.Marshal(event)
			if err != nil {
				n.logger.Printf("encode tag request: %v", err)
				continue
			}
			select {
			case out <- alloy.Job{Type: "nottager.tag.requested", Source: n.Id(), Payload: payload}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

type UpdateTags struct{ logger *log.Logger }

func (n *UpdateTags) Id() string                  { return "update-tags" }
func (n *UpdateTags) NumInstances() int           { return 1 }
func (n *UpdateTags) Init(s alloy.Services) error { n.logger = s.Logger; return nil }
func (n *UpdateTags) Start(ctx context.Context, workerID string, in <-chan alloy.Job, _ chan<- alloy.Job) {
	n.logger.Printf("starting %s", workerID)
	for {
		select {
		case job, ok := <-in:
			if !ok {
				return
			}
			var event pageEvent
			if err := json.Unmarshal(job.Payload, &event); err != nil {
				n.logger.Printf("decode tag request: %v", err)
				continue
			}
			// This is deliberately a dry-run placeholder for the former AI + Notion
			// integration: demonstrate the final stage without external side effects.
			n.logger.Printf("Nottager dry run: page=%q (%s), content_length=%d", event.Title, event.PageID, len(event.Content))
		case <-ctx.Done():
			return
		}
	}
}
