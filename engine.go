package main

import (
	"context"
	"fmt"
	"net/http"
)

type Flow interface {
	Run(ctx context.Context, payload map[string]any) 
}

type Trigger interface {
	Id() string
	Send(ctx context.Context, payload chan <- Job)
}

type Job struct {
	Source string
	Payload map[string]any
}

type Engine struct {
	HttpClient    *http.Client    // Used for polling/triggers that make outbound HTTP requests
	HttpServerMux *http.ServeMux  // Used for incoming webhooks
	Triggers      []Trigger
	Router        map[string]Flow
	Jobs          chan Job // channel buffer size can be variable
}

func (e *Engine) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, t := range e.Triggers {
		go t.Send(ctx, e.Jobs)
	}

	if e.HttpServerMux != nil {
		go func() {
			if err := http.ListenAndServe(":8080", e.HttpServerMux); err != nil {
				fmt.Println("bruhhh")
			}
		}()
	}

	for {
		for j := range e.Jobs {
			fmt.Println(j.Source)
			go e.Router[j.Source].Run(ctx, j.Payload)
		}
	}
	
}


func (e *Engine) Shutdown() {}