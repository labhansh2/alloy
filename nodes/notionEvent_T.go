package nodes

import (
	"alloy"
	"alloy/clients/notion"
	"context"
	"log"
	"net/http"
	"os"
)

const notionWebhookPath = "/notion-event"

type NotionEvent struct {
	serverMux *http.ServeMux
	logger    *log.Logger
}

func (n *NotionEvent) NumInstances() int { return 1 }

func (n *NotionEvent) Id() string {
	return "NotionEvent"
}

func (n *NotionEvent) Init(s alloy.Services) error {
	n.serverMux = s.HttpServerMux
	n.logger = s.Logger
	return nil
}

func (n *NotionEvent) Start(ctx context.Context, workerId string, _ <-chan alloy.Job, outJobs chan<- alloy.Job) {

	n.logger.Printf("starting node worker %s\n", workerId)

	secret := os.Getenv("NOTION_WH_VERIFICATION_TOKEN")

	wh := alloy.NewWebhook(ctx, n.serverMux, notionWebhookPath,
		alloy.RequiresAuth(),
		alloy.WithWebhookLogger(n.logger),
		alloy.WithWebhookVerify(notion.RequestVerifier(secret, n.logger)),
	)

	for {
		select {
		case data := <-wh.C:
			n.logger.Printf("recieved a notion event")
			outJobs <- alloy.Job{
				Source:  n.Id(),
				Payload: data,
			}
		case <-ctx.Done():
			n.logger.Printf("shutting down node worker %s\n", workerId)
			return
		}
	}
}
