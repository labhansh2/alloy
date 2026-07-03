package main

import (
	"alloy"
	"context"
	"log"
	"net/http"
)

type TestNode2 struct {
	httpClient *http.Client
	logger     *log.Logger
}

func (t *TestNode2) Id() string { return "TestNode2" }

func (t *TestNode2) NumInstances() int { return 1}

func (t *TestNode2) Init(services alloy.Services) {
	t.httpClient = services.HttpClient
	t.logger = services.Logger
}

func (t *TestNode2) Start(ctx context.Context, inJob <-chan alloy.Job, _ chan<- alloy.Job) {
	for j := range inJob {
		t.logger.Printf("Received %v\n", j.Payload)
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
}
