package flows

import (
	"alloy"
	"context"
	"log"
)

type BangFlow struct {
	logger *log.Logger
}

func (BF *BangFlow) Id() string {
	return "BangFlow"
}

func (BF *BangFlow) Init(s alloy.Services) {
	BF.logger = s.Logger
}

func (BT *BangFlow) Run(ctx context.Context, payload alloy.Payload) {
	BT.logger.Println(payload)
}