package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"alloy/clients"
)

const BaseURL = "https://openrouter.ai/api/v1"

// Client is the shared HTTP client configured for OpenRouter.
type Client clients.Client

func WithReferer(referer string) clients.Option {
	return clients.WithHeader("HTTP-Referer", referer)
}

func WithTitle(title string) clients.Option {
	return clients.WithHeader("X-OpenRouter-Title", title)
}

func New(token string, opts ...clients.Option) *Client {
	all := append([]clients.Option{
		clients.WithErrorDecoder(decodeAPIError),
	}, opts...)
	return (*Client)(clients.New(token, BaseURL, all...))
}

// NewAIClient is an alias for New kept for alloy call sites.
func NewAIClient(httpClient *http.Client, token string, opts ...clients.Option) *Client {
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
	var envelope struct {
		Error APIError `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error.Message == "" {
		return fmt.Errorf("ai: http %d: %s", status, string(body))
	}
	envelope.Error.StatusCode = status
	return &envelope.Error
}
