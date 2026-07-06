package notion

import (
	"context"
	"encoding/json"
	"net/http"
)

// DataSource is a table under a Notion database (API 2025-09-03+).
type DataSource struct {
	Object         string                     `json:"object"`
	ID             string                     `json:"id"`
	Properties     map[string]json.RawMessage `json:"properties"`
	Parent         Parent                     `json:"parent"`
	DatabaseParent Parent                     `json:"database_parent"`
	CreatedTime    string                     `json:"created_time"`
	LastEditedTime string                     `json:"last_edited_time"`
	CreatedBy      UserRef                    `json:"created_by"`
	LastEditedBy   UserRef                    `json:"last_edited_by"`
	Title          []RichText                 `json:"title"`
	Description    []RichText                 `json:"description"`
	Icon           json.RawMessage            `json:"icon"`
	InTrash        bool                       `json:"in_trash"`
}

// GetDataSource retrieves a data source schema.
func (c *Client) GetDataSource(ctx context.Context, dataSourceID string) (*DataSource, error) {
	req, err := c.NewRequest(http.MethodGet, "/v1/data_sources/"+dataSourceID, nil, nil)
	if err != nil {
		return nil, err
	}
	var ds DataSource
	if err := c.Do(ctx, req, &ds); err != nil {
		return nil, err
	}
	return &ds, nil
}

type QueryDataSourceParams struct {
	Filter      json.RawMessage `json:"filter,omitempty"`
	Sorts       json.RawMessage `json:"sorts,omitempty"`
	StartCursor string          `json:"start_cursor,omitempty"`
	PageSize    int             `json:"page_size,omitempty"`
}

// QueryDataSource returns pages in a data source.
func (c *Client) QueryDataSource(ctx context.Context, dataSourceID string, params QueryDataSourceParams) (*List[Page], error) {
	req, err := c.NewRequest(http.MethodPost, "/v1/data_sources/"+dataSourceID+"/query", nil, params)
	if err != nil {
		return nil, err
	}
	var list List[Page]
	if err := c.Do(ctx, req, &list); err != nil {
		return nil, err
	}
	return &list, nil
}
