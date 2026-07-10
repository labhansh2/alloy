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

	engine, _ := alloy.NewEngine(alloy.Services{})

	n1 := &TestNode1{}
	n2 := &TestNode2{}
	n3 := &TestNode3{}
	n4 := &TestNode4{}
	n5 := &TestNode5{}
	n6 := &TestNode6{}

	if err := engine.RegisterNodes([]alloy.Node{n1, n2, n3, n4, n5, n6}); err != nil {
		log.Fatal(err)
	}

	if err := engine.RegisterConnection(n4.Id(), n1.Id()); err != nil {
		log.Fatal(err)
	}
	if err := engine.RegisterConnection(n4.Id(), n3.Id()); err != nil {
		log.Fatal(err)
	}
	if err := engine.RegisterConnection(n5.Id(), n2.Id()); err != nil {
		log.Fatal(err)
	}
	if err := engine.RegisterConnection(n6.Id(), n3.Id()); err != nil {
		log.Fatal(err)
	}

	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer engine.Shutdown()
}
