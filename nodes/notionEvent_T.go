package nodes

import "alloy"

type NotionEvent struct {
	wh alloy.Webhook
}

func (n *NotionEvent) Id() string {
	return "NotionEvent"
}

func (n *NotionEvent) Init() {
	// wh := alloy.NewWebhook()
}
