package main

import (
	"alloy"
	"context"
	"log"
	"time"
)

type TestNode4 struct {
	logger *log.Logger
}

func (t *TestNode4) Id() string {
	return "TestNode4"
}

func (t *TestNode4) NumInstances() int { return 1 }

func (t *TestNode4) Init(services alloy.Services) {
	t.logger = services.Logger
}

func (t *TestNode4) Start(ctx context.Context, workerId string, _ <-chan alloy.Job, outJob chan<- alloy.Job) {
	p := map[string]any{
		"something": "something else",
	}

	thisJob := alloy.Job{
		Source:  t.Id(),
		Payload: p,
	}

	ticker := time.NewTicker(2 * time.Second)

	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("[%s] %s: context cancelled, shutting down", workerId, t.Id())
			return
		case <-ticker.C:
			t.logger.Printf("[%s] %s: emitting job with payload: %v", workerId, t.Id(), thisJob.Payload)
			outJob <- thisJob
		}
	}
}
