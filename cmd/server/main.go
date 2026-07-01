package main

import (

	"os"
	"os/signal"
	"syscall"
	"context"
	"log"

	"alloy"
	"alloy/flows"
	"alloy/triggers"
)

func main() {

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	engine := alloy.NewEngine()

	engine.RegisterFlow(&triggers.BangTrigger{}, &flows.BangFlow{})

	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
}