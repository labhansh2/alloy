package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"alloy"
	"alloy/clients/ai"
	"alloy/clients/notion"
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

	notionToken := os.Getenv("NOTION_INTEGRATION_TOKEN")
	if notionToken == "" {
		log.Fatal("NOTION_INTEGRATION_TOKEN is required")
	}
	aiToken := os.Getenv("OPENROUTER_TOKEN")
	if aiToken == "" {
		log.Fatal("OPENROUTER_TOKEN is required")
	}

	notion := notion.New(notionToken)
	ai := ai.New(aiToken)

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

	detect := &nodes.DetectPageUpdate{Notion: notion}
	updateTags := &nodes.UpdateTags{Notion:notion, Ai: ai}

	if err := engine.RegisterNodes([]alloy.Node{notionEvent, detect, updateTags}); err != nil {
		log.Fatal(err)
	}
	if err := engine.RegisterConnection(notionEvent.Id(), detect.Id()); err != nil {
		log.Fatal(err)
	}
	if err := engine.RegisterConnection(detect.Id(), updateTags.Id()); err != nil {
		log.Fatal(err)
	}

	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer engine.Shutdown()
}
