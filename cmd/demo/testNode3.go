package main

import (
	"alloy"
	"context"
	"log"
)

type TestNode3 struct {
	logger *log.Logger
}

func (t *TestNode3) Id() string { return "TestFlow" }

func (t *TestNode3) NumInstances() int { return 1}

func (t *TestNode3) Init(Services alloy.Services) {
	t.logger = Services.Logger
}

func (t *TestNode3) Start(ctx context.Context, inJob <-chan alloy.Job, _ chan<- alloy.Job) {

	for j := range inJob {
		t.logger.Printf("Node 3: with payload: %v\n", j.Payload)
	}
}
