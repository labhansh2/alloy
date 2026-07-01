package main

import (
	"alloy"
	"context"
	"log"
	"time"
)

type TestTrigger struct {
	logger *log.Logger
}

func (t *TestTrigger) Id() string {
	return "testTrigger"
}

func (t *TestTrigger) Init(Services alloy.Services) {
	t.logger = Services.Logger
	t.logger.Printf("Initializing %s", t.Id())
}

func (t *TestTrigger) Start(ctx context.Context, job chan<- alloy.Job) {

	t.logger.Printf("%s started", t.Id())

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
			return
		case <-ticker.C:
			job <- thisJob
		}
	}

}
