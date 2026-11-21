// Command prototype-node exercises the node connection primitives from a
// separate process, as a node in another environment would.
package main

import (
	"context"
	"log"
	"time"

	"alloy/nodewire"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	node, err := nodewire.Dial(ctx, "ws://127.0.0.1:8080/node", "prototype-node")
	if err != nil {
		log.Fatal(err)
	}
	defer node.Close()
	var result map[string]any
	if err := node.CallResource(ctx, "echo", map[string]string{"message": "hello from node"}, &result); err != nil {
		log.Fatal(err)
	}
	log.Printf("server resource result: %#v", result)
	if err := node.Publish(ctx, map[string]string{"kind": "prototype.completed"}); err != nil {
		log.Fatal(err)
	}
	log.Println("node communication proof complete")
}
