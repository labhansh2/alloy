// Command prototype-node exercises the node connection primitives from a
// separate process, as a node in another environment would.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/url"
	"os"
	"time"

	"alloy/internal/prototype/nodewire"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	path := flag.String("connection", "", "server-generated cli-node-connection.json file")

	flag.Parse()

	if *path == "" {
		return errors.New("provide -connection with the shared server's CLI node connection file")
	}

	data, err := os.ReadFile(*path)
	if err != nil {
		return errors.New("cannot read CLI node connection file")
	}

	var connection struct {
		Endpoint string `json:"endpoint"`
		Token    string `json:"token"`
	}
	if json.Unmarshal(data, &connection) != nil || connection.Token == "" {
		return errors.New("invalid CLI node connection file")
	}

	u, err := url.Parse(connection.Endpoint)
	if err != nil || u.Scheme != "ws" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.Path != "/node" || u.User != nil || u.RawQuery != "" {
		return errors.New("expected a loopback Alloy node endpoint")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	node, err := nodewire.Dial(ctx, connection.Endpoint, "prototype-node", connection.Token)
	if err != nil {
		return err
	}
	defer node.Close()

	var result map[string]any
	if err := node.CallResource(ctx, "echo", map[string]string{"text": "hello from node"}, &result); err != nil {
		return err
	}

	log.Printf("shared echo CLI result: %#v", result["output"])

	if err := node.CallResource(ctx, "ls", nil, &result); err != nil {
		return err
	}

	log.Printf("shared ls CLI result:\n%v", result["output"])

	if err := node.Publish(ctx, map[string]string{"kind": "prototype.completed"}); err != nil {
		return err
	}

	log.Println("node communication proof complete")

	return nil
}
