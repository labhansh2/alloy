package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"alloy"
	"alloy/clients/notion"
)

type DetectPageUpdate struct {
	httpClient *http.Client
	logger     *log.Logger
	notion     *notion.Client

	pageCache map[string]string // dirty pages
}

func (d *DetectPageUpdate) Id() string { return "DetectPageUpdate" }

func (d *DetectPageUpdate) NumInstances() int { return 1 }

func (d *DetectPageUpdate) Init(s alloy.Services) error {
	d.pageCache = make(map[string]string)
	d.httpClient = s.HttpClient
	d.logger = s.Logger

	c, ok := s.Clients["notion"].(*notion.Client)
	if !ok || c == nil {
		return errors.New("'notion' client not registered in the engine")
	}
	d.notion = c
	return nil
}

func (d *DetectPageUpdate) Start(ctx context.Context, workerId string, inJob <-chan alloy.Job, outJob chan<- alloy.Job) {
	d.logger.Printf("starting node worker %s", workerId)
	for {
		select {
		case data := <-inJob: // notion event
			var event notion.WebhookPayload
			if err := json.Unmarshal(data.Payload, &event); err != nil {
				fmt.Println("invalid notion event")
				continue
			}
			d.logger.Println(event)

			if event.Entity.Type == "page" {
				pageId := event.Entity.Id
				pageContent, err := d.notion.RetrievePageMarkdown(ctx, pageId)
				if err != nil {
					d.logger.Printf("failed to retrieve page %s: %v", pageId, err)
					continue
				}
				d.pageCache[pageId] = pageContent.Markdown
			}
		case <-ctx.Done():
			d.logger.Printf("shutting down node worker %s", workerId)
			return
		}
	}
}
