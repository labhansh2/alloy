package alloy

import (
	"context"
	"net/http"
	"time"

	"alloy/clients"
)

type Poll struct {
	C chan []byte
}

func NewPoll(
	ctx context.Context,
	client *clients.Client,
	req *http.Request,
	pollDuration time.Duration,
) *Poll {
	p := Poll{C: make(chan []byte)}
	go p.run(ctx, client, req, pollDuration)
	return &p
}

func NewPollWithBuffer(
	ctx context.Context,
	client *clients.Client,
	req *http.Request,
	pollDuration time.Duration,
	bufferSize int,
) *Poll {
	p := Poll{C: make(chan []byte, bufferSize)}
	go p.run(ctx, client, req, pollDuration)
	return &p
}

func (p *Poll) run(
	ctx context.Context,
	client *clients.Client,
	req *http.Request,
	pollDuration time.Duration,
) {
	t := time.NewTicker(pollDuration)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			data, err := client.DoRaw(ctx, req)
			if err != nil {
				continue
			}
			select {
			case p.C <- data:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
