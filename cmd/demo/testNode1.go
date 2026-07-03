package main

import (
	"alloy"
	"context"
	"log"
)

type TestNode1 struct {
	logger *log.Logger
}

func (t *TestNode1) Id() string { return "TestNode" }

func (t *TestNode1) NumInstances() int { return 1 }

func (t *TestNode1) Init(Services alloy.Services) {
	t.logger = Services.Logger
}

func (t *TestNode1) Start(ctx context.Context, inJob <-chan alloy.Job, _ chan<- alloy.Job) {
	for j := range inJob {
		t.logger.Printf("Node 1: with payload: %v\n", j.Payload)
	}
}
