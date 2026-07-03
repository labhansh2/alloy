package main

import (
	"alloy"
	"context"
	"encoding/json"
	"log"
	"net/http"
)

type testTrigger2 struct {
	logger        *log.Logger
	httpServerMux *http.ServeMux
}

func (t *testTrigger2) Id() string {
	return "testTrigger2"
}

func (t *testTrigger2) Init(services alloy.Services) {
	t.logger = services.Logger
	t.httpServerMux = services.HttpServerMux
}

func (t *testTrigger2) Start(ctx context.Context, job chan<- alloy.Job) {

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
			job <- alloy.Job{Source: t.Id(), Payload: data}
		}
	}
}
