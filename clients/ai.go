package clients

import "net/http"

type AI struct {
	client http.Client
}

func NewAIClient(client *http.Client) *AI {
	return &AI{
		client: *client,
	}
}
