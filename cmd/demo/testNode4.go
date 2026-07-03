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

func (t *TestNode4) Init(Services alloy.Services) {
	t.logger = Services.Logger
	t.logger.Printf("Initializing %s", t.Id())
}

func (t *TestNode4) Start(ctx context.Context, _ <-chan alloy.Job, outJob chan<- alloy.Job) {

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
			t.logger.Printf("%s stopped", t.Id())
			return
		case <-ticker.C:
			outJob <- thisJob
		}
	}

}
