package main

import (
	"context"
	"log"
)

type TestFlow3 struct {
	logger *log.Logger
}

func (t *TestFlow3) Id() string { return "TestFlow" }

func (t *TestFlow3) Init(Services Services) {
	t.logger = Services.Logger
}

func (t *TestFlow3) Run(ctx context.Context, payload map[string]any) {
	t.logger.Printf("running flow 3 with payload: %v\n", payload)
}
