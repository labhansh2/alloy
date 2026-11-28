// Command prototype-event-emitter emits a job directly or through the server's
// HTTP trigger, then waits for its correlated completion event.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"alloy/internal/prototype/events"
	"alloy/internal/prototype/nodewire"
)

type connection struct {
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	path := flag.String("connection", "", "server-generated emitter-node-connection.json")
	trigger := flag.String("trigger", "", "optional trigger-connection.json; start the job via HTTP instead of Emit")
	text := flag.String("text", "hello from an event", "job input")

	flag.Parse()

	connection, err := load(*path)
	if err != nil {
		return err
	}

	u, err := url.Parse(connection.Endpoint)
	if err != nil || u.Scheme != "ws" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.Path != "/events/emitter" || u.User != nil || u.RawQuery != "" {
		return errors.New("expected the loopback Alloy emitter endpoint")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	node, err := nodewire.Dial(ctx, connection.Endpoint, "prototype-event-emitter", connection.Token)
	if err != nil {
		return err
	}
	defer node.Close()

	completed, err := node.Subscribe(ctx, "demo.completed")
	if err != nil {
		return err
	}

	var receipt events.Receipt
	if *trigger == "" {
		receipt, err = node.Emit(ctx, "demo.requested", map[string]string{"text": *text})
	} else {
		receipt, err = fireHTTP(ctx, *trigger, u.Host, *text)
	}

	if err != nil {
		return err
	}

	if receipt.Queued == 0 || receipt.Dropped > 0 {
		return errors.New("no worker accepted this ephemeral job; start the worker first")
	}

	log.Printf("Event %s admitted to %d server queue(s); waiting for processing", receipt.EventID, receipt.Queued)

	for {
		event, err := completed.Next(ctx)
		if err != nil {
			return err
		}

		var result struct {
			RequestID string `json:"requestId"`
			Status    string `json:"status"`
			Result    string `json:"result"`
			Error     string `json:"error"`
		}
		if json.Unmarshal(event.Payload, &result) != nil || result.RequestID != receipt.EventID {
			continue
		}

		if result.Status != "succeeded" {
			return fmt.Errorf("worker reported failure: %s", result.Error)
		}

		fmt.Printf("Completed by %s: %s\n", event.Source, result.Result)

		return nil
	}
}

func load(path string) (connection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return connection{}, errors.New("cannot read the server-generated connection file")
	}

	var c connection
	if json.Unmarshal(data, &c) != nil || c.Token == "" {
		return connection{}, errors.New("invalid connection file")
	}

	return c, nil
}

func fireHTTP(ctx context.Context, path, serverHost, text string) (events.Receipt, error) {
	c, err := load(path)
	if err != nil {
		return events.Receipt{}, err
	}

	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme != "http" || u.Host != serverHost || u.Path != "/hooks/demo" || u.User != nil || u.RawQuery != "" {
		return events.Receipt{}, errors.New("trigger must belong to the same loopback Alloy server")
	}

	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return events.Receipt{}, err
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return events.Receipt{}, err
	}

	r.Header.Set("Authorization", "Bearer "+c.Token)
	r.Header.Set("Content-Type", "application/json")

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	response, err := client.Do(r)
	if err != nil {
		return events.Receipt{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		return events.Receipt{}, fmt.Errorf("HTTP trigger rejected the job (status %d); check its credential and worker readiness", response.StatusCode)
	}

	var receipt events.Receipt
	if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil {
		return events.Receipt{}, err
	}

	return receipt, nil
}
