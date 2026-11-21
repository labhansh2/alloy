// Command prototype-node exercises the node connection primitives from a
// separate process, as a node in another environment would.
package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"alloy/internal/prototype/nodewire"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url := os.Getenv("ALLOY_PROTOTYPE_URL")
	if url == "" {
		url = "ws://127.0.0.1:8080/node"
	}
	if strings.HasPrefix(url, "http://") {
		url = "ws" + strings.TrimPrefix(url, "http")
	}
	node, err := nodewire.Dial(ctx, url, "prototype-node")
	if err != nil {
		log.Fatal(err)
	}
	defer node.Close()
	var result map[string]any
	if err := node.CallResource(ctx, "echo", map[string]string{"text": "hello from node"}, &result); err != nil {
		log.Fatal(err)
	}
	log.Printf("shared echo CLI result: %#v", result["output"])
	if err := node.CallResource(ctx, "ls", nil, &result); err != nil {
		log.Fatal(err)
	}
	log.Printf("shared ls CLI result:\n%v", result["output"])
	if err := node.Publish(ctx, map[string]string{"kind": "prototype.completed"}); err != nil {
		log.Fatal(err)
	}
	log.Println("node communication proof complete")
}
