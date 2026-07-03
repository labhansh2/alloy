package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"alloy"
	// "alloy/nodes"
)

func main() {

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	engine := alloy.NewEngine()

	// engine.RegisterFlow(&triggers.DetectBang{}, &flows.BangFlow{})

	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
