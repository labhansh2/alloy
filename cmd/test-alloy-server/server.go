// Command test-alloy-server runs a local, credential-free smoke test of the
// current in-process engine using the former Nottager flow as an example.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	testnodes "alloy/cmd/test-alloy-server/nodes"
	alloy "alloy/engine"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	engine, err := alloy.NewEngine(alloy.Services{})
	if err != nil {
		log.Fatal(err)
	}

	nodes := []alloy.Node{
		&testnodes.NotionWebhookNode{},
		&testnodes.DetectPageUpdate{},
		&testnodes.UpdateTags{},
	}
	if err := engine.RegisterNodes(nodes); err != nil {
		log.Fatal(err)
	}
	if err := engine.Subscribe("notion.page.updated", "detect-page-update"); err != nil {
		log.Fatal(err)
	}
	if err := engine.Subscribe("nottager.tag.requested", "update-tags"); err != nil {
		log.Fatal(err)
	}

	log.Println("test server listening at http://localhost:8080")
	log.Println(`POST /test/notion-event with JSON: {"page_id":"page-123","title":"Example","content":"A note to tag"}`)
	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
	engine.Shutdown()
}
