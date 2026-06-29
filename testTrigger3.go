package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

type TestTrigger3 struct {
	logger     *log.Logger
	httpClient *http.Client
}

func (t *TestTrigger3) Id() string { return "TestTrigger3" }

func (t *TestTrigger3) Init(services Services) {
	t.logger = services.Logger
	t.httpClient = services.HttpClient
}

func (t *TestTrigger3) Start(ctx context.Context, job chan<- Job) {

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

	p := NewPoll(ctx, t.httpClient, req, 2*time.Second)

	for {
		select {
		case data := <-p.C:
			m := map[string]any{"post": string(data)}
			job <- Job{
				Source:  t.Id(),
				Payload: m,
			}
		case <-ctx.Done():
			return
		}
	}
}
