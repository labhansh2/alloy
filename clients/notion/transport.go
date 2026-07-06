package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) newRequest(method, path string, query url.Values, body any) (*http.Request, error) {
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
	req.Header.Set("Notion-Version", c.version)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) do(ctx context.Context, req *http.Request, dst any) error {
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
		return decodeAPIError(resp.StatusCode, data)
	}
	if dst == nil {
		return nil
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("notion: decode response: %w", err)
	}
	return nil
}

func decodeAPIError(status int, data []byte) error {
	var apiErr APIError
	if err := json.Unmarshal(data, &apiErr); err != nil {
		return fmt.Errorf("notion: http %d: %s", status, string(data))
	}
	apiErr.StatusCode = status
	return &apiErr
}
