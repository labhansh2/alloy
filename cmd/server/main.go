package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"alloy"
	"alloy/nodes"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	engine, err := alloy.NewEngine(
		alloy.Services{},
		alloy.WithTunneling(ctx, &alloy.TunnelCfg{
			Authtoken: os.Getenv("NGROK_AUTHTOKEN"),
			Domain:    os.Getenv("NGROK_DOMAIN"),
		}),
	)

	if err != nil {
		log.Fatal(err)
	}

	notionEvent := &nodes.NotionEvent{}
	pageUpdate := &nodes.DetectPageUpdate{}

	engine.RegisterNodes([]alloy.Node{notionEvent, pageUpdate})
	engine.RegisterConnection(notionEvent.Id(), pageUpdate.Id())

	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer engine.Shutdown()
}
