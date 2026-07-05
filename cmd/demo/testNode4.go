package main

import (
	"alloy"
	"context"
	"encoding/json"
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

	b, _ := json.Marshal(p)

	thisJob := alloy.Job{
		Source:  t.Id(),
		Payload: b,
	}

	ticker := time.NewTicker(2 * time.Second)

	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("shutting down node worker %s\n", workerId)
			return
		case <-ticker.C:
			outJob <- thisJob
		}
	}
}
