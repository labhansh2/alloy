package main

import (
	"alloy"
	"context"
	"encoding/json"
	"log"
)

type TestNode1 struct {
	logger *log.Logger
}

func (t *TestNode1) Id() string { return "TestNode" }

func (t *TestNode1) NumInstances() int { return 1 }

func (t *TestNode1) Init(services alloy.Services) {
	t.logger = services.Logger
}

func (t *TestNode1) Start(ctx context.Context, workerId string, inJob <-chan alloy.Job, _ chan<- alloy.Job) {

	type someStruct struct {
		Something string `json:"something"`
	}

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
			var s someStruct
			e := json.Unmarshal(j.Payload, &s)
			if e != nil {
				t.logger.Println("bruh bruh bruh")
			}

			t.logger.Printf("[%s] %s: received job with payload: %v", workerId, t.Id(), s)
		}
	}
}
