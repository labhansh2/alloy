package main

import (
	"context"
	"time"
)

type testTrigger struct {}

func (t *testTrigger) Id() string {
	return "testTrigger"
}

func (t *testTrigger) Send(ctx context.Context, job chan <- Job) {

	p := map[string]any {
		"something": "something else",
	}

	thisJob := Job{
		Source: t.Id(),
		Payload: p,
	}

	ticker := time.NewTicker(2 * time.Second)
	for {
		select{
		case <- ctx.Done(): return 
		case <- ticker.C: job <- thisJob
		}
	}

}

