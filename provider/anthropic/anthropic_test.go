package anthropic_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"alloy/node"
	"alloy/provider"
	"alloy/provider/anthropic"
	"alloy/server"

	native "github.com/anthropics/anthropic-sdk-go/v2"
)

func TestProviderConfiguration(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	config := anthropic.Provider()
	if config.BaseURL != "https://api.anthropic.com" || config.Login != nil {
		t.Fatal("unexpected Anthropic registration")
	}

	if err := config.CheckAuth(context.Background()); err == nil {
		t.Fatal("missing key accepted")
	}

	t.Setenv("ANTHROPIC_API_KEY", "server-anthropic-key")

	if err := config.CheckAuth(context.Background()); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest("POST", "/v1/messages", nil)
	if err := config.Auth(request); err != nil || request.Header.Get("X-Api-Key") != "server-anthropic-key" {
		t.Fatal("server API key was not applied")
	}
}

func TestNativeClientThroughAlloy(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "server-anthropic-key")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "ambient-auth-token")
	t.Setenv("ANTHROPIC_PROFILE", "must-not-load")
	t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("ANTHROPIC_CUSTOM_HEADERS", "Authorization: Bearer ambient-secret\nX-Ambient-Secret: must-not-forward")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != "server-anthropic-key" {
			t.Error("native route or server API key was lost")
		}

		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Ambient-Secret") != "" {
			t.Error("Alloy or ambient authority leaked upstream")
		}

		if r.Header.Get("Anthropic-Version") == "" {
			t.Error("native API-version header missing")
		}

		var body struct {
			Model     string `json:"model"`
			Stream    bool   `json:"stream"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}

		if body.Model != "test-model" || body.MaxTokens != 32 {
			t.Error("native request parameters were changed")
		}

		if body.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"test-model\",\"content\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"streamed Claude\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"msg_test","type":"message","role":"assistant","model":"test-model","content":[{"type":"text","text":"hello from Claude"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":3}}`)
	}))
	defer upstream.Close()

	connection := startAnthropicServer(t, upstream.URL)
	client := anthropic.Client(connection)
	params := native.MessageNewParams{
		Model:     "test-model",
		MaxTokens: 32,
		Messages:  []native.MessageParam{native.NewUserMessage(native.NewTextBlock("hello"))},
	}

	result, err := client.Messages.New(context.Background(), params)
	if err != nil || len(result.Content) != 1 || result.Content[0].Text != "hello from Claude" {
		t.Fatalf("native message failed: %v", err)
	}

	stream := client.Messages.NewStreaming(context.Background(), params)
	defer stream.Close()

	text := ""

	for stream.Next() {
		event := stream.Current()
		if event.Type == "content_block_delta" {
			text += event.Delta.Text
		}
	}

	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}

	if text != "streamed Claude" {
		t.Fatal("native stream did not arrive")
	}
}

func TestClientUsesOnlyAlloyCredentials(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "ambient-secret")
	t.Setenv("ANTHROPIC_CUSTOM_HEADERS", "X-Ambient-Secret: must-not-forward")

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer alloy-client-token" ||
			r.Header.Get("X-Api-Key") != "" || r.Header.Get("X-Ambient-Secret") != "" {
			t.Error("SDK used ambient credentials instead of the Alloy connection")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"msg_test","type":"message","role":"assistant","model":"test-model","content":[],"usage":{"input_tokens":1,"output_tokens":0}}`)
	}))
	defer gateway.Close()

	client := anthropic.Client(provider.NewConnection(gateway.URL+"/", "alloy-client-token"))
	if _, err := client.Messages.New(context.Background(), native.MessageNewParams{
		Model: "test-model", MaxTokens: 32,
		Messages: []native.MessageParam{native.NewUserMessage(native.NewTextBlock("hello"))},
	}); err != nil {
		t.Fatal(err)
	}
}

func startAnthropicServer(t *testing.T, upstream string) provider.Connection {
	t.Helper()

	config := anthropic.Provider()
	config.BaseURL = upstream

	srv := server.New()
	if err := srv.AddProvider("anthropic", config); err != nil {
		t.Fatal(err)
	}

	token := strings.Repeat("a", 32)
	if err := srv.AddClient("test", server.Client{Token: token, Providers: []string{"anthropic"}}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan error, 1)

	go func() { done <- srv.Run(ctx) }()

	t.Cleanup(func() {
		cancel()

		if err := <-done; err != nil {
			t.Error(err)
		}
	})

	select {
	case <-srv.Ready():
	case <-ctx.Done():
		t.Fatal("server did not become ready")
	}

	n, err := node.New(node.Config{Name: "test", ServerURL: "http://" + srv.Addr(), ClientToken: token})
	if err != nil {
		t.Fatal(err)
	}

	connection, err := n.Provider(ctx, "anthropic")
	if err != nil {
		t.Fatal(err)
	}

	return connection
}

func TestNativeErrorsAreNotRetried(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "server-secret")

	var calls atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`)
	}))
	defer upstream.Close()

	connection := startAnthropicServer(t, upstream.URL)
	client := anthropic.Client(connection)

	_, err := client.Messages.New(context.Background(), native.MessageNewParams{
		Model: "test-model", MaxTokens: 32,
		Messages: []native.MessageParam{native.NewUserMessage(native.NewTextBlock("hello"))},
	})
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("native upstream error was not preserved: %v", err)
	}

	if calls.Load() != 1 {
		t.Fatalf("native request retried %d times", calls.Load()-1)
	}
}
