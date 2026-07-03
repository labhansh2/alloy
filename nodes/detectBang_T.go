package nodes

import (
	"alloy"
	"alloy/clients/notion"

	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type DetectBang struct {
	logger  *log.Logger
	httpMux *http.ServeMux
	notion  *notion.Notion

	pages     []string          // pages to track for bang
	pageCache map[string]string // dirty pages
}

func (d *DetectBang) Id() string {
	return "DetectBang"
}

func (d *DetectBang) Init(s alloy.Services) {
	d.logger = s.Logger
	d.httpMux = s.HttpServerMux
	d.notion = notion.NewNotionClient(s.HttpClient)

	d.pages = make([]string, 10)
	d.pageCache = make(map[string]string)
}

func (d *DetectBang) Start(ctx context.Context, payload chan<- alloy.Job) {

	pageUpdated := alloy.NewWebhook(ctx, d.httpMux, "/notion")
	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case dataBytes := <-pageUpdated.C:
			var event notion.WebhookPayload
			if err := json.Unmarshal(dataBytes, &event); err != nil {
				d.logger.Fatalf("Failed to unmarshal webhook data: %v", err)
				continue
			}

			if event.Type == notion.EventPageCreated || event.Type == notion.EventPageContentUpdated {
				d.pageCache[event.Entity.Id] = d.notion.getPageContent(event.Entity.Id)
			}

		case <-ticker.C:

		case <-ctx.Done():
			d.logger.Printf("%s stopeed", d.Id())
			return
		}
	}
}
