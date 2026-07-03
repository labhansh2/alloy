package main

import (
	"alloy"
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

func (t *TestNode6) NumInstances() int { return  1}

func (t *TestNode6) Init(services alloy.Services) {
	t.logger = services.Logger
	t.httpClient = services.HttpClient
}

func (t *TestNode6) Start(ctx context.Context, _ <-chan alloy.Job, outJob chan<- alloy.Job) {

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://jsonplaceholder.typicode.com/posts/1",
		nil,
	)
	if err != nil {
		panic(err)
	}

	req.Header.Set("User-Agent", "MyBot/1.0")
	req.Header.Set("Accept", "application/json")

	p := alloy.NewPoll(ctx, t.httpClient, req, 2*time.Second)

	for {
		select {
		case data := <-p.C:
			m := map[string]any{"post": string(data)}
			outJob <- alloy.Job{
				Source:  t.Id(),
				Payload: m,
			}
		case <-ctx.Done():
			return
		}
	}
}
