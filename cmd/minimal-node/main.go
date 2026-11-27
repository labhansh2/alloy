// Command minimal-node connects a standalone Go program to a local Alloy server.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"alloy/node"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	serverURL := flag.String("server", "", "local Alloy server base URL")
	name := flag.String("name", "minimal-node", "descriptive node name")

	flag.Parse()

	n, err := node.New(node.Config{Name: *name, ServerURL: *serverURL})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return n.Run(ctx)
}
