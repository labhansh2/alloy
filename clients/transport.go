package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is shared JSON-over-HTTP transport for REST API clients.
type Client struct {
	http        *http.Client
	token       string
	baseURL     string
	headers     map[string]string
	decodeError func(status int, body []byte) error
}

type Option func(*Client)

func WithHTTPClient(c *http.Client) Option {
	return func(client *Client) {
		if c != nil {
			client.http = c
		}
	}
}

func WithHeader(key, value string) Option {
	return func(client *Client) {
		if key != "" {
			client.headers[key] = value
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

func WithErrorDecoder(fn func(status int, body []byte) error) Option {
	return func(client *Client) {
		client.decodeError = fn
	}
}

func New(token, baseURL string, opts ...Option) *Client {
	c := &Client{
		http:    http.DefaultClient,
		token:   token,
		baseURL: baseURL,
		headers: make(map[string]string),
		decodeError: func(status int, body []byte) error {
			return fmt.Errorf("http %d: %s", status, string(body))
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) SetHTTPClient(h *http.Client) {
	if h != nil {
		c.http = h
	}
}

func (c *Client) SetBaseURL(baseURL string) {
	if baseURL != "" {
		c.baseURL = baseURL
	}
}

func (c *Client) SetHeader(key, value string) {
	if key == "" {
		return
	}
	if value == "" {
		delete(c.headers, key)
		return
	}
	c.headers[key] = value
}

// NewRequest builds an authenticated request against the client's base URL.
func (c *Client) NewRequest(method, path string, query url.Values, body any) (*http.Request, error) {
	u, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, u, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// Do executes a request and decodes a JSON response into dst.
func (c *Client) Do(ctx context.Context, req *http.Request, dst any) error {
	if ctx != nil {
		req = req.WithContext(ctx)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return c.decodeError(resp.StatusCode, data)
	}
	if dst == nil {
		return nil
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
