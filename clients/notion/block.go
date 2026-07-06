package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// Block is a minimal block object; use Raw for type-specific fields.
type Block struct {
	Object         string          `json:"object"`
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	HasChildren    bool            `json:"has_children"`
	InTrash        bool            `json:"in_trash"`
	CreatedTime    string          `json:"created_time"`
	LastEditedTime string          `json:"last_edited_time"`
	Raw            json.RawMessage `json:"-"`
}

func (b *Block) UnmarshalJSON(data []byte) error {
	var raw struct {
		Object         string `json:"object"`
		ID             string `json:"id"`
		Type           string `json:"type"`
		HasChildren    bool   `json:"has_children"`
		InTrash        bool   `json:"in_trash"`
		CreatedTime    string `json:"created_time"`
		LastEditedTime string `json:"last_edited_time"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.Object = raw.Object
	b.ID = raw.ID
	b.Type = raw.Type
	b.HasChildren = raw.HasChildren
	b.InTrash = raw.InTrash
	b.CreatedTime = raw.CreatedTime
	b.LastEditedTime = raw.LastEditedTime
	b.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type GetBlockChildrenParams struct {
	StartCursor string
	PageSize    int
}

// GetBlockChildren returns child blocks of a page or block.
func (c *Client) GetBlockChildren(ctx context.Context, blockID string, params GetBlockChildrenParams) (*List[Block], error) {
	query := url.Values{}
	if params.StartCursor != "" {
		query.Set("start_cursor", params.StartCursor)
	}
	if params.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(params.PageSize))
	}
	req, err := c.newRequest(http.MethodGet, "/v1/blocks/"+blockID+"/children", query, nil)
	if err != nil {
		return nil, err
	}
	var list List[Block]
	if err := c.do(ctx, req, &list); err != nil {
		return nil, err
	}
	return &list, nil
}
