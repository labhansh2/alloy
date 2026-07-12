package ai

import (
	"encoding/json"
	"fmt"
	"net/http"

	"alloy/clients"
)

const BaseURL = "https://openrouter.ai/api/v1"

// Client talks to the OpenRouter API.
type Client struct {
	*clients.Client
	token string
}

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
	return &Client{
		Client: clients.New(token, BaseURL, all...),
		token:  token,
	}
}

func NewAIClient(httpClient *http.Client, token string, opts ...clients.Option) *Client {
	all := append([]clients.Option{clients.WithHTTPClient(httpClient)}, opts...)
	return New(token, all...)
}

func (c *Client) Token() string {
	return c.token
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
