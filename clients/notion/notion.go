package notion

import "net/http"

type Notion struct {
	client *http.Client
}

func NewNotionClient(client *http.Client) *Notion {
	return &Notion{
		client: client,
	}
}