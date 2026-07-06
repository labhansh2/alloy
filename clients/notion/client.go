package notion

import (
	"net/http"
)

const (
	BaseURL    = "https://api.notion.com"
	APIVersion = "2026-03-11"
)

// Client talks to the Notion REST API.
type Client struct {
	http    *http.Client
	token   string
	version string
	baseURL string
}

type Option func(*Client)

func WithHTTPClient(c *http.Client) Option {
	return func(client *Client) {
		if c != nil {
			client.http = c
		}
	}
}

func WithAPIVersion(version string) Option {
	return func(client *Client) {
		if version != "" {
			client.version = version
		}
	}
}

func WithBaseURL(baseURL string) Option {
	return func(client *Client) {
		if baseURL != "" {
			client.baseURL = baseURL
		}
	}
}

func New(token string, opts ...Option) *Client {
	c := &Client{
		http:    http.DefaultClient,
		token:   token,
		version: APIVersion,
		baseURL: BaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewNotionClient is an alias for New kept for alloy call sites.
func NewNotionClient(client *http.Client, token string, opts ...Option) *Client {
	all := append([]Option{WithHTTPClient(client)}, opts...)
	return New(token, all...)
}