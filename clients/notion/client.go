package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"alloy/clients"
)

const (
	BaseURL    = "https://api.notion.com"
	APIVersion = "2026-03-11"
)

// Client is the shared HTTP client configured for the Notion API.
type Client clients.Client

func WithAPIVersion(version string) clients.Option {
	return clients.WithHeader("Notion-Version", version)
}

func New(token string, opts ...clients.Option) *Client {
	all := append([]clients.Option{
		clients.WithHeader("Notion-Version", APIVersion),
		clients.WithErrorDecoder(decodeAPIError),
	}, opts...)
	return (*Client)(clients.New(token, BaseURL, all...))
}

// NewNotionClient is an alias for New kept for alloy call sites.
func NewNotionClient(httpClient *http.Client, token string, opts ...clients.Option) *Client {
	all := append([]clients.Option{clients.WithHTTPClient(httpClient)}, opts...)
	return New(token, all...)
}

func (c *Client) api() *clients.Client {
	return (*clients.Client)(c)
}

func (c *Client) NewRequest(method, path string, query url.Values, body any) (*http.Request, error) {
	return c.api().NewRequest(method, path, query, body)
}

func (c *Client) Do(ctx context.Context, req *http.Request, dst any) error {
	return c.api().Do(ctx, req, dst)
}

func decodeAPIError(status int, body []byte) error {
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("notion: http %d: %s", status, string(body))
	}
	apiErr.StatusCode = status
	return &apiErr
}
