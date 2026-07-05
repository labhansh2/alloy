package main

import (
	"alloy"
	"context"
	"log"
	"time"
)

type TestNode3 struct {
	logger *log.Logger
}

func (t *TestNode3) Id() string { return "TestFlow" }

func (t *TestNode3) NumInstances() int { return 1 }

func (t *TestNode3) Init(services alloy.Services) {
	t.logger = services.Logger
}

func (t *TestNode3) Start(ctx context.Context, workerId string, inJob <-chan alloy.Job, _ chan<- alloy.Job) {
	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("[%s] %s: context cancelled, shutting down", workerId, t.Id())
			return
		case j, ok := <-inJob:
			if !ok {
				t.logger.Printf("[%s] %s: input channel closed, shutting down", workerId, t.Id())
				return
			}
			time.Sleep(3 * time.Second)
			t.logger.Printf("[%s] %s: received job with payload: %v", workerId, t.Id(), j.Payload)
		}
	}
}
