package notion

import (
	"encoding/json"
	"fmt"
	"net/http"

	"alloy/clients"
)

const (
	BaseURL    = "https://api.notion.com"
	APIVersion = "2026-03-11"
)

// Client talks to the Notion REST API.
type Client struct {
	*clients.Client
	token string
}

func WithAPIVersion(version string) clients.Option {
	return clients.WithHeader("Notion-Version", version)
}

func New(token string, opts ...clients.Option) *Client {
	all := append([]clients.Option{
		clients.WithHeader("Notion-Version", APIVersion),
		clients.WithErrorDecoder(decodeAPIError),
	}, opts...)
	return &Client{
		Client: clients.New(token, BaseURL, all...),
		token:  token,
	}
}

func NewNotionClient(httpClient *http.Client, token string, opts ...clients.Option) *Client {
	all := append([]clients.Option{clients.WithHTTPClient(httpClient)}, opts...)
	return New(token, all...)
}

func (c *Client) Token() string {
	return c.token
}

func decodeAPIError(status int, body []byte) error {
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("notion: http %d: %s", status, string(body))
	}
	apiErr.StatusCode = status
	return &apiErr
}
