package main

import (
	"alloy"
	"context"
	"encoding/json"
	"log"
	"net/http"
)

type TestNode5 struct {
	logger        *log.Logger
	httpServerMux *http.ServeMux
}

func (t *TestNode5) Id() string {
	return "TestNode5"
}

func (t *TestNode5) NumInstances() int { return 1 }

func (t *TestNode5) Init(services alloy.Services) {
	t.logger = services.Logger
	t.httpServerMux = services.HttpServerMux
}

func (t *TestNode5) Start(ctx context.Context, _ <-chan alloy.Job, outJob chan<- alloy.Job) {

	wh := alloy.NewWebhook(ctx, t.httpServerMux, "/some")

	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("%s stopped", t.Id())
			return
		case dataBytes := <-wh.C:
			var data map[string]any
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				t.logger.Fatalf("Failed to unmarshal webhook data: %v", err)
				continue
			}
			outJob <- alloy.Job{Source: t.Id(), Payload: data}
		}
	}
}
