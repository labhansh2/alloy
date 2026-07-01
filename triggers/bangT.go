package triggers

import (
	"alloy"
	"context"
	"math/rand"
	"time"
)

type BangTrigger struct {}

func (BT *BangTrigger) Id() string {
	return "BangTrigger"
}

func (BT *BangTrigger) Init(s alloy.Services) {}

func (BT *BangTrigger) Start(ctx context.Context, payload chan<- alloy.Job) {
	
	ticker := time.NewTicker(time.Second * 2)
	for {
		select{
		case <- ticker.C :
			time.Sleep(2 * time.Second)
			randInt := rand.Int()
			payload <- alloy.Job{Source: BT.Id(), Payload: map[string]any{"n": randInt}}
		case <- ctx.Done(): return
		}
	}
}