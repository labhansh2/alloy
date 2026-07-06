package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Page is a Notion page object (properties only; use block children for content).
type Page struct {
	Object         string              `json:"object"`
	ID             string              `json:"id"`
	CreatedTime    string              `json:"created_time"`
	LastEditedTime string              `json:"last_edited_time"`
	CreatedBy      UserRef             `json:"created_by"`
	LastEditedBy   UserRef             `json:"last_edited_by"`
	Cover          json.RawMessage     `json:"cover"`
	Icon           json.RawMessage     `json:"icon"`
	Parent         Parent              `json:"parent"`
	InTrash        bool                `json:"in_trash"`
	Properties     map[string]Property `json:"properties"`
	URL            string              `json:"url"`
	PublicURL      *string             `json:"public_url"`
}

type GetPageParams struct {
	FilterProperties []string
}

// GetPage retrieves a page by ID.
func (c *Client) GetPage(ctx context.Context, pageID string, params GetPageParams) (*Page, error) {
	query := url.Values{}
	for _, prop := range params.FilterProperties {
		query.Add("filter_properties", prop)
	}
	req, err := c.NewRequest(http.MethodGet, "/v1/pages/"+pageID, query, nil)
	if err != nil {
		return nil, err
	}
	var page Page
	if err := c.Do(ctx, req, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

type UpdatePageParams struct {
	Properties map[string]any `json:"properties,omitempty"`
	Icon       any            `json:"icon,omitempty"`
	Cover      any            `json:"cover,omitempty"`
	InTrash    *bool          `json:"in_trash,omitempty"`
}

// UpdatePage updates a page's properties, icon, cover, or trash status.
func (c *Client) UpdatePage(ctx context.Context, pageID string, params UpdatePageParams) (*Page, error) {
	req, err := c.NewRequest(http.MethodPatch, "/v1/pages/"+pageID, nil, params)
	if err != nil {
		return nil, err
	}
	var page Page
	if err := c.Do(ctx, req, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

type PageMarkdown struct {
	Object   string `json:"object"`
	ID       string `json:"id"`
	Markdown string `json:"markdown"`
}

// RetrievePageMarkdown returns a page's content as enhanced markdown.
func (c *Client) RetrievePageMarkdown(ctx context.Context, pageID string) (*PageMarkdown, error) {
	req, err := c.NewRequest(http.MethodGet, "/v1/pages/"+pageID+"/markdown", nil, nil)
	if err != nil {
		return nil, err
	}
	var md PageMarkdown
	if err := c.Do(ctx, req, &md); err != nil {
		return nil, err
	}
	return &md, nil
}
