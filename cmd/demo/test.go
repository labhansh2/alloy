package main

import (
	"alloy"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	engine := alloy.NewEngine()

	engine.RegisterFlow(&TestTrigger{}, &TestFlow{})
	engine.RegisterFlow(&testTrigger2{}, &TestFlow2{})
	engine.RegisterFlow(&TestTrigger3{}, &TestFlow3{})

	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
