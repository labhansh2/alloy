package main

import (
	"context"
	"fmt"
)

type testFlow struct{}

func (t *testFlow) Run(ctx context.Context, payload map[string]any) {
	fmt.Printf("running flow with payload: %v\n", payload)
}
