package main

import (
	"context"
	"io"
	"net/http"
	"time"
)

type Poll struct {
	C chan []byte
}

func NewPoll(ctx context.Context, httpClient *http.Client, req *http.Request, pollDuration time.Duration) *Poll {
	p := Poll{
		C: make(chan []byte),
	}
	go p.run(ctx, httpClient, req, pollDuration)

	return &p
}

func (p *Poll) run(ctx context.Context, httpClient *http.Client, req *http.Request, pollDuration time.Duration) {

	t := time.NewTicker(pollDuration)

	for {
		select {
		case <-t.C:
			resp, err := httpClient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			// this can be improved
			data, _ := io.ReadAll(resp.Body)
			p.C <- data

		case <-ctx.Done():
			return
		}
	}
}
