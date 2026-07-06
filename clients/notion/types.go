package notion

import (
	"encoding/json"
	"fmt"
)

// APIError is returned for non-2xx responses.
type APIError struct {
	StatusCode int    `json:"-"`
	Object     string `json:"object"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("notion: %s (%s)", e.Message, e.Code)
	}
	return fmt.Sprintf("notion: %s", e.Message)
}

// List is a paginated Notion list response.
type List[T any] struct {
	Object     string `json:"object"`
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
	Type       string `json:"type"`
}

type UserRef struct {
	Object string `json:"object"`
	ID     string `json:"id"`
}

type Parent struct {
	Type          string `json:"type"`
	PageID        string `json:"page_id,omitempty"`
	DatabaseID    string `json:"database_id,omitempty"`
	DataSourceID  string `json:"data_source_id,omitempty"`
	Workspace     bool   `json:"workspace,omitempty"`
	BlockID       string `json:"block_id,omitempty"`
}

type RichText struct {
	Type        string          `json:"type"`
	Text        *RichTextText   `json:"text,omitempty"`
	Annotations RichTextAnnot   `json:"annotations"`
	PlainText   string          `json:"plain_text"`
	Href        *string         `json:"href"`
}

type RichTextText struct {
	Content string  `json:"content"`
	Link    *string `json:"link"`
}

type RichTextAnnot struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

// Property is a page property value. Use Raw to decode type-specific payloads.
type Property struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (p *Property) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.ID = raw.ID
	p.Type = raw.Type
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}
