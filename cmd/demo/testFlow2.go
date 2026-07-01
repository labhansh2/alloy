package main

import (
	"alloy"
	"context"
	"log"
	"net/http"
)

type TestFlow2 struct {
	httpClient *http.Client
	logger     *log.Logger
}

func (t *TestFlow2) Id() string { return "TestFlow2" }

func (t *TestFlow2) Init(services alloy.Services) {
	t.httpClient = services.HttpClient
	t.logger = services.Logger
}

func (t *TestFlow2) Run(ctx context.Context, payload map[string]any) {
	t.logger.Printf("Received %v\n", payload)
	resp, err := t.httpClient.Get("http://localhost:8000/")
	if err != nil {
		t.logger.Println("Error fetching from localhost:8000:", err)
		return
	}
	defer resp.Body.Close()

	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.logger.Println("Error reading response body:", err)
		return
	}
	t.logger.Printf("Response from localhost:8000: %s\n", string(buf[:n]))
}
