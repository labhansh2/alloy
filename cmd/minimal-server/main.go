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
	done := make(chan struct{})

	go func() {
		defer close(done)

		select {
		case <-srv.Ready():
			log.Printf("Alloy server: http://%s", srv.Addr())
		case <-ctx.Done():
		}
	}()

	defer func() {
		stop()
		<-done
	}()

	return srv.Run(ctx)
}
