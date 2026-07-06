package ai

import "fmt"

// APIError is returned for non-2xx responses.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       any    `json:"code"`
	Message    string `json:"message"`
	Metadata   any    `json:"metadata,omitempty"`
}

func (e *APIError) Error() string {
	if e.Code != nil {
		return fmt.Sprintf("ai: %s (%v)", e.Message, e.Code)
	}
	return fmt.Sprintf("ai: %s", e.Message)
}

// Message is a chat message. Content is typically a string for text prompts.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
