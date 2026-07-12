package main

import (
	"alloy"
	"context"
	"encoding/json"
	"log"
)

type TestNode3 struct {
	logger *log.Logger
}

func (t *TestNode3) Id() string { return "TestFlow" }

func (t *TestNode3) NumInstances() int { return 1 }

func (t *TestNode3) Init(services alloy.Services) error {
	t.logger = services.Logger
	return nil
}

func (t *TestNode3) Start(ctx context.Context, workerId string, inJob <-chan alloy.Job, _ chan<- alloy.Job) {
	type someStruct struct {
		Something string `json:"something"`
	}

	type post struct {
		Body   string `json:"body"`
		Id     string `json:"id"`
		Title  string `json:"title"`
		UserId string `json:"userId"`
	}

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

			switch j.Source {
			case "TestNode6":
				var m post
				e := json.Unmarshal(j.Payload, &m)
				if e != nil {
				}
				t.logger.Printf("[%s] %s: received job with payload: %v", workerId, t.Id(), m)
			case "TestNode4":
				var s someStruct
				e := json.Unmarshal(j.Payload, &s)
				if e != nil {
					t.logger.Println("bruh bruh bruh")
				}
				t.logger.Printf("[%s] %s: received job with payload: %v", workerId, t.Id(), s)
			}

		}
	}
}
