package main

import (
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

func (t *TestTrigger) Init(Services Services) {
	t.logger = Services.Logger
	t.logger.Printf("Initializing %s", t.Id())
}

func (t *TestTrigger) Start(ctx context.Context, job chan<- Job) {

	t.logger.Printf("%s started", t.Id())

	p := map[string]any{
		"something": "something else",
	}

	thisJob := Job{
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
