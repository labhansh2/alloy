package main

import (
	"alloy"
	"context"
	"log"
)

type TestFlow struct {
	logger *log.Logger
}

func (t *TestFlow) Id() string { return "TestFlow" }

func (t *TestFlow) Init(Services alloy.Services) {
	t.logger = Services.Logger
}

func (t *TestFlow) Run(ctx context.Context, payload alloy.Payload) {
	t.logger.Printf("TestFlow: running flow with payload: %v\n", payload)
}
