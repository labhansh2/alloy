package notion

import (
	"context"
	"encoding/json"
	"net/http"
)

type SearchParams struct {
	Query       string          `json:"query,omitempty"`
	Sort        json.RawMessage `json:"sort,omitempty"`
	Filter      json.RawMessage `json:"filter,omitempty"`
	StartCursor string          `json:"start_cursor,omitempty"`
	PageSize    int             `json:"page_size,omitempty"`
}

// Search returns pages and data sources shared with the integration.
func (c *Client) Search(ctx context.Context, params SearchParams) (*List[json.RawMessage], error) {
	req, err := c.newRequest(http.MethodPost, "/v1/search", nil, params)
	if err != nil {
		return nil, err
	}
	var list List[json.RawMessage]
	if err := c.do(ctx, req, &list); err != nil {
		return nil, err
	}
	return &list, nil
}
