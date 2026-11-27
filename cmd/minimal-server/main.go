// Command minimal-server runs an Alloy server using the public Go SDK.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"alloy/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New()

	return srv.Run(ctx)
}
