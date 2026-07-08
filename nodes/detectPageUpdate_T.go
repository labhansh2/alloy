package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"alloy"
	"alloy/clients/notion"
)

type DetectPageUpdate struct {
	httpClient *http.Client
	logger     *log.Logger

	pageCache map[string]string // dirty pages
}

func (d *DetectPageUpdate) Id() string { return "DetectPageUpdate" }

func (d *DetectPageUpdate) NumInstances() int { return 1 }

func (d *DetectPageUpdate) Init(s alloy.Services) {
	d.httpClient = s.HttpClient
	d.logger = s.Logger
}

func (d *DetectPageUpdate) Start(ctx context.Context, workerId string, inJob <-chan alloy.Job, outJob chan<- alloy.Job) {
	d.logger.Printf("starting node worker %s", workerId)
	for {
		select {
		case data := <-inJob: // notion event
			var event notion.WebhookPayload
			e := json.Unmarshal(data.Payload, &event)
			if e != nil {
				fmt.Println("invalid notion event")
			}
			d.logger.Println(event)
		case <-ctx.Done():
			d.logger.Printf("shutting down node worker %s", workerId)
			return
		}
	}
}
