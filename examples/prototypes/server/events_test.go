package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alloy/internal/prototype/events"
	"alloy/internal/prototype/nodewire"
)

func TestDemoTriggerValidationAndAdmission(t *testing.T) {
	b := events.NewBroker(1)
	handler := demoTrigger(b, "trigger-token")

	request := func(token, body string) int {
		r := httptest.NewRequest(http.MethodPost, "/hooks/demo", bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		return w.Code
	}
	if status := request("wrong", `{"text":"hello"}`); status != 401 {
		t.Fatalf("unauthenticated trigger: %d", status)
	}

	for _, body := range []string{`{`, `{"text":""}`, `{"text":"hello","subject":"arbitrary"}`, `{"text":"hello"}{}`} {
		if status := request("trigger-token", body); status != 400 {
			t.Errorf("invalid input accepted: %d %s", status, body)
		}
	}

	if status := request("trigger-token", `{"text":"hello"}`); status != 503 {
		t.Fatalf("offline worker was acknowledged: %d", status)
	}

	sub, err := b.Subscribe("worker", "demo.requested")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	if status := request("trigger-token", `{"text":"hello"}`); status != 202 {
		t.Fatalf("ready worker not admitted: %d", status)
	}

	if status := request("trigger-token", `{"text":"full"}`); status != 503 {
		t.Fatalf("full queue was acknowledged: %d", status)
	}
}

func TestRegisteredEventNodesAndHTTPTrigger(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	state := filepath.Join(t.TempDir(), "private")
	mux := http.NewServeMux()

	host := httptest.NewServer(mux)
	defer host.Close()

	if err := registerEvents(mux, host.URL, state); err != nil {
		t.Fatal(err)
	}

	type connection struct {
		Endpoint string `json:"endpoint"`
		Token    string `json:"token"`
	}

	load := func(name string) connection {
		t.Helper()

		data, err := os.ReadFile(filepath.Join(state, name))
		if err != nil {
			t.Fatal(err)
		}

		var c connection
		if err := json.Unmarshal(data, &c); err != nil {
			t.Fatal(err)
		}

		return c
	}
	e := load("emitter-node-connection.json")
	w := load("worker-node-connection.json")
	trigger := load("trigger-connection.json")

	emitter, err := nodewire.Dial(ctx, e.Endpoint, "prototype-event-emitter", e.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer emitter.Close()

	worker, err := nodewire.Dial(ctx, w.Endpoint, "prototype-event-worker", w.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()

	completed, err := emitter.Subscribe(ctx, "demo.completed")
	if err != nil {
		t.Fatal(err)
	}

	jobs, err := worker.Subscribe(ctx, "demo.requested")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := worker.Emit(ctx, "demo.requested", nil); err == nil {
		t.Fatal("worker obtained publisher authority for requested jobs")
	}

	if err := emitter.CallResource(ctx, "echo", nil, nil); err == nil {
		t.Fatal("emitter obtained worker's resource grant")
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, trigger.Endpoint, bytes.NewBufferString(`{"text":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("Authorization", "Bearer "+trigger.Token)

	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != 202 {
		t.Fatalf("HTTP trigger failed: %d", response.StatusCode)
	}

	var receipt events.Receipt
	if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil {
		t.Fatal(err)
	}

	job, err := jobs.Next(ctx)
	if err != nil || job.Source != "prototype-http-trigger" || job.ID != receipt.EventID {
		t.Fatalf("incorrect HTTP event: %+v %v", job, err)
	}

	if _, err := worker.Emit(ctx, "demo.completed", map[string]string{"requestId": job.ID, "status": "succeeded"}); err != nil {
		t.Fatal(err)
	}

	event, err := completed.Next(ctx)
	if err != nil || event.Source != "prototype-event-worker" {
		t.Fatalf("completion failed: %+v %v", event, err)
	}
}
