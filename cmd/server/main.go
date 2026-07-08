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

	services := alloy.Services{}
	if token := os.Getenv("NGROK_AUTHTOKEN"); token != "" {
		services.Tunnel = &alloy.NgrokTunnel{
			Authtoken: token,
			Domain:    os.Getenv("NGROK_DOMAIN"),
		}
	}

	engine := alloy.NewEngineWithServices(services)

	notionEvent := &nodes.NotionEvent{}
	pageUpdate := &nodes.DetectPageUpdate{}

	engine.RegisterNodes([]alloy.Node{notionEvent, pageUpdate})

	engine.RegisterConnection(notionEvent.Id(), pageUpdate.Id())

	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer engine.Shutdown()
}
