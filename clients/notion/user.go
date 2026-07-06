package notion

import (
	"context"
	"net/http"
)

// User is a Notion user (person or bot).
type User struct {
	Object    string `json:"object"`
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email,omitempty"`
}

// GetSelf returns the bot user for the configured token.
func (c *Client) GetSelf(ctx context.Context) (*User, error) {
	req, err := c.newRequest(http.MethodGet, "/v1/users/me", nil, nil)
	if err != nil {
		return nil, err
	}
	var user User
	if err := c.do(ctx, req, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUser retrieves a user by ID.
func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	req, err := c.newRequest(http.MethodGet, "/v1/users/"+userID, nil, nil)
	if err != nil {
		return nil, err
	}
	var user User
	if err := c.do(ctx, req, &user); err != nil {
		return nil, err
	}
	return &user, nil
}
