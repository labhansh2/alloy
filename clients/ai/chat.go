package ai

import (
	"context"
	"encoding/json"
	"net/http"
)

type ChatCompletionParams struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    *float64        `json:"temperature,omitempty"`
	MaxTokens      *int            `json:"max_tokens,omitempty"`
	TopP           *float64        `json:"top_p,omitempty"`
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`
}

type ChatCompletion struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

type ChatChoice struct {
	Index        int     `json:"index"`
	FinishReason string  `json:"finish_reason"`
	Message      Message `json:"message"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Content returns the assistant text from the first choice, if any.
func (c *ChatCompletion) Content() string {
	if len(c.Choices) == 0 {
		return ""
	}
	return c.Choices[0].Message.Content
}

// ChatCompletion sends a chat completion request.
func (c *Client) ChatCompletion(ctx context.Context, params ChatCompletionParams) (*ChatCompletion, error) {
	req, err := c.NewRequest(http.MethodPost, "/chat/completions", nil, params)
	if err != nil {
		return nil, err
	}
	var completion ChatCompletion
	if err := c.Do(ctx, req, &completion); err != nil {
		return nil, err
	}
	return &completion, nil
}
