package ai

import (
	"context"
	"net/http"
)

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ContextLen  int    `json:"context_length"`
}

type ModelList struct {
	Data []Model `json:"data"`
}

// ListModels returns models available on OpenRouter.
func (c *Client) ListModels(ctx context.Context) (*ModelList, error) {
	req, err := c.NewRequest(http.MethodGet, "/models", nil, nil)
	if err != nil {
		return nil, err
	}
	var list ModelList
	if err := c.Do(ctx, req, &list); err != nil {
		return nil, err
	}
	return &list, nil
}
