package main

import (
	"alloy"
	"alloy/clients"
	"context"
	"log"
	"net/http"
	"time"
)

type TestNode6 struct {
	logger     *log.Logger
	httpClient *http.Client
}

func (t *TestNode6) Id() string { return "TestNode6" }

func (t *TestNode6) NumInstances() int { return 1 }

func (t *TestNode6) Init(services alloy.Services) {
	t.logger = services.Logger
	t.httpClient = services.HttpClient
}

func (t *TestNode6) Start(ctx context.Context, workerId string, _ <-chan alloy.Job, outJob chan<- alloy.Job) {
	c := clients.New("", "https://jsonplaceholder.typicode.com", clients.WithHTTPClient(t.httpClient))
	req, err := c.NewRequest(http.MethodGet, "/posts/1", nil, nil)
	if err != nil {
		panic(err)
	}

	p := alloy.NewPoll(ctx, c, req, 2*time.Second)

	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("[%s] %s: context cancelled, shutting down", workerId, t.Id())
			return
		case data, ok := <-p.C:
			if !ok {
				t.logger.Printf("[%s] %s: poll channel closed, shutting down", workerId, t.Id())
				return
			}
			job := alloy.Job{
				Source:  t.Id(),
				Payload: data,
			}
			outJob <- job
		}
	}
}
