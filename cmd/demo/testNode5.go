package main

import (
	"alloy"
	"context"
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

func (t *TestNode5) Init(services alloy.Services) error {
	t.logger = services.Logger
	t.httpServerMux = services.HttpServerMux
	return nil
}

func (t *TestNode5) Start(ctx context.Context, workerId string, _ <-chan alloy.Job, outJob chan<- alloy.Job) {
	wh := alloy.NewWebhook(ctx, t.httpServerMux, "/some")

	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("[%s] %s: context cancelled, shutting down", workerId, t.Id())
			return
		case dataBytes, ok := <-wh.C:
			if !ok {
				t.logger.Printf("[%s] %s: webhook channel closed, shutting down", workerId, t.Id())
				return
			}
			// var data map[string]any
			// if err := json.Unmarshal(dataBytes, &data); err != nil {
			// 	t.logger.Printf("[%s] %s: error unmarshaling webhook data: %v", workerId, t.Id(), err)
			// 	continue
			// }
			job := alloy.Job{Source: t.Id(), Payload: dataBytes}
			// t.logger.Printf("[%s] %s: emitting job with payload: %v", workerId, t.Id(), job.Payload)
			outJob <- job
		}
	}
}
