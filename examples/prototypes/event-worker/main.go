// Command prototype-event-worker receives ephemeral jobs, calls a shared server
// resource over the same connection, and emits correlated completion reports.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"alloy/internal/prototype/events"
	"alloy/internal/prototype/nodewire"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	path := flag.String("connection", "", "server-generated worker-node-connection.json")
	once := flag.Bool("once", false, "process one job and exit")

	flag.Parse()

	data, err := os.ReadFile(*path)
	if err != nil {
		return errors.New("provide -connection with the worker node connection file")
	}

	var connection struct {
		Endpoint string `json:"endpoint"`
		Token    string `json:"token"`
	}
	if json.Unmarshal(data, &connection) != nil || connection.Token == "" {
		return errors.New("invalid worker connection file")
	}

	u, err := url.Parse(connection.Endpoint)
	if err != nil || u.Scheme != "ws" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.Path != "/events/worker" || u.User != nil || u.RawQuery != "" {
		return errors.New("expected the loopback Alloy worker endpoint")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	node, err := nodewire.Dial(ctx, connection.Endpoint, "prototype-event-worker", connection.Token)
	if err != nil {
		return err
	}
	defer node.Close()

	jobs, err := node.Subscribe(ctx, "demo.requested")
	if err != nil {
		return err
	}

	log.Println("Worker ready: subscribed to demo.requested")

	for {
		event, err := jobs.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return err
		}

		log.Printf("Received event %s from %s", event.ID, event.Source)
		result := process(ctx, node, event)

		receipt, err := node.Emit(ctx, "demo.completed", result)
		if err != nil {
			return err
		}

		log.Printf("Completion emitted for %s: queued=%d dropped=%d", event.ID, receipt.Queued, receipt.Dropped)

		if *once {
			return nil
		}
	}
}

func process(ctx context.Context, node *nodewire.Client, event events.Event) map[string]string {
	result := map[string]string{"requestId": event.ID, "status": "failed"}

	var input struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(event.Payload, &input) != nil || strings.TrimSpace(input.Text) == "" || len(input.Text) > 4096 {
		result["error"] = "expected nonempty text of at most 4096 bytes"
		return result
	}

	var echo struct {
		Output string `json:"output"`
	}
	if err := node.CallResource(ctx, "echo", map[string]string{"text": strings.ToUpper(input.Text)}, &echo); err != nil {
		result["error"] = "shared echo resource failed"
		return result
	}

	result["status"] = "succeeded"
	result["result"] = strings.TrimSuffix(echo.Output, "\n")

	return result
}
