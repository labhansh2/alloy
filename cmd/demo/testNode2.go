package main

import (
	"alloy"
	"context"
	"io"
	"log"
	"net/http"
)

type TestNode2 struct {
	httpClient *http.Client
	logger     *log.Logger
}

func (t *TestNode2) Id() string { return "TestNode2" }

func (t *TestNode2) NumInstances() int { return 1 }

func (t *TestNode2) Init(services alloy.Services) {
	t.httpClient = services.HttpClient
	t.logger = services.Logger
}

func (t *TestNode2) Start(ctx context.Context, workerId string, inJob <-chan alloy.Job, _ chan<- alloy.Job) {
	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("[%s] %s: context cancelled, shutting down", workerId, t.Id())
			return
		case j, ok := <-inJob:
			if !ok {
				t.logger.Printf("[%s] %s: input channel closed, shutting down", workerId, t.Id())
				return
			}
			t.logger.Printf("[%s] %s: received job with payload: %v", workerId, t.Id(), j.Payload)
			resp, err := t.httpClient.Get("http://localhost:8000/")
			if err != nil {
				t.logger.Printf("[%s] %s: error fetching from localhost:8000: %v", workerId, t.Id(), err)
				return
			}
			func() {
				defer resp.Body.Close()
				buf := make([]byte, 1024)
				n, err := resp.Body.Read(buf)
				if err != nil && err != io.EOF {
					t.logger.Printf("[%s] %s: error reading response body: %v", workerId, t.Id(), err)
					return
				}
				t.logger.Printf("[%s] %s: response from localhost:8000: %s", workerId, t.Id(), string(buf[:n]))
			}()
		}
	}
}
