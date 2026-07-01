package main

import (
	"alloy"
	"context"
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
			return
		case data := <-wh.C:
			job <- alloy.Job{Source: t.Id(), Payload: data}
		}
	}
}
