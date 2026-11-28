package google_test

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
	"alloy/provider/google"
	"alloy/server"

	"google.golang.org/genai"
)

func TestProviderConfiguration(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")

	config := google.Provider()
	if config.BaseURL != "https://generativelanguage.googleapis.com" || config.Login != nil {
		t.Fatal("unexpected Google registration")
	}

	if err := config.CheckAuth(context.Background()); err == nil {
		t.Fatal("missing key accepted")
	}

	t.Setenv("GEMINI_API_KEY", "gemini-secret")

	if err := config.CheckAuth(context.Background()); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest("POST", "/v1beta/models/test:generateContent?key=caller&access_token=caller&alt=sse", nil)
	request.Header.Set("X-Goog-User-Project", "caller-project")

	if err := config.Auth(request); err != nil {
		t.Fatal(err)
	}

	if request.Header.Get("X-Goog-Api-Key") != "gemini-secret" ||
		request.Header.Get("X-Goog-User-Project") != "" ||
		request.URL.Query().Has("key") || request.URL.Query().Has("access_token") ||
		request.URL.Query().Get("alt") != "sse" {
		t.Fatal("Google authentication did not own upstream authority")
	}

	t.Setenv("GOOGLE_API_KEY", "google-secret")

	if err := config.Auth(request); err != nil || request.Header.Get("X-Goog-Api-Key") != "google-secret" {
		t.Fatal("GOOGLE_API_KEY did not take precedence")
	}
}

func TestNativeClientThroughAlloy(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "server-google-key")
	t.Setenv("GEMINI_API_KEY", "unused-google-key")
	t.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "true")
	t.Setenv("GOOGLE_GEMINI_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("GOOGLE_GENAI_API_VERSION", "invalid-ambient-version")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Goog-Api-Key") != "server-google-key" || r.Header.Get("Authorization") != "" {
			t.Error("upstream did not receive only the server Google credential")
		}

		if r.URL.Query().Has("key") || r.URL.Query().Has("access_token") || r.Header.Get("X-Goog-User-Project") != "" {
			t.Error("caller query credentials or billing selection reached Google")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil || !json.Valid(body) || !strings.Contains(string(body), "hello") {
			t.Error("native request body was not forwarded")
		}

		switch r.URL.Path {
		case "/v1beta/models/test-model:generateContent":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello from Gemini"}]},"finishReason":"STOP"}]}`)
		case "/v1beta/models/test-model:streamGenerateContent":
			if r.URL.Query().Get("alt") != "sse" {
				t.Error("native streaming query was lost")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"streamed Gemini\"}]},\"finishReason\":\"STOP\"}]}\n\n")
		default:
			t.Errorf("unexpected native route: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	connection := startGoogleServer(t, upstream.URL)

	client, err := google.Client(context.Background(), connection)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Models.GenerateContent(context.Background(), "test-model", genai.Text("hello"), nil)
	if err != nil || result.Text() != "hello from Gemini" {
		t.Fatalf("native generation failed: %v", err)
	}

	text := ""

	for chunk, err := range client.Models.GenerateContentStream(context.Background(), "test-model", genai.Text("hello"), nil) {
		if err != nil {
			t.Fatal(err)
		}

		text += chunk.Text()
	}

	if text != "streamed Gemini" {
		t.Fatal("native stream did not arrive")
	}

	// Non-SDK callers cannot override upstream authority via Google's alternate
	// credential inputs, even with a valid Alloy grant.
	request, err := http.NewRequest(http.MethodPost,
		connection.BaseURL()+"v1beta/models/test-model:generateContent?key=caller-secret&access_token=caller-token",
		strings.NewReader(`{"contents":[{"parts":[{"text":"hello"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}

	request.Header.Set("Authorization", "Bearer "+connection.Token())
	request.Header.Set("X-Goog-Api-Key", "caller-secret")
	request.Header.Set("X-Goog-User-Project", "caller-project")

	response, err := connection.HTTPClient().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("custom native request failed: %d", response.StatusCode)
	}
}

func TestClientUsesOnlyAlloyCredentials(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "ambient-secret")

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer alloy-client-token" ||
			r.Header.Get("X-Goog-Api-Key") != "alloy-gateway" || r.URL.Query().Has("key") {
			t.Error("native SDK used ambient credentials instead of the Alloy connection")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"candidates":[]}`)
	}))
	defer gateway.Close()

	client, err := google.Client(context.Background(), provider.NewConnection(gateway.URL+"/", "alloy-client-token"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Models.GenerateContent(context.Background(), "test-model", genai.Text("hello"), nil); err != nil {
		t.Fatal(err)
	}
}

func startGoogleServer(t *testing.T, upstream string) provider.Connection {
	t.Helper()

	config := google.Provider()
	config.BaseURL = upstream

	srv := server.New()
	if err := srv.AddProvider("google", config); err != nil {
		t.Fatal(err)
	}

	token := strings.Repeat("g", 32)
	if err := srv.AddClient("test", server.Client{Token: token, Providers: []string{"google"}}); err != nil {
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

	connection, err := n.Provider(ctx, "google")
	if err != nil {
		t.Fatal(err)
	}

	return connection
}

func TestNativeErrorsAreNotRetried(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "server-secret")

	var calls atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"code":429,"message":"rate limited","status":"RESOURCE_EXHAUSTED"}}`)
	}))
	defer upstream.Close()

	connection := startGoogleServer(t, upstream.URL)

	client, err := google.Client(context.Background(), connection)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Models.GenerateContent(context.Background(), "test-model", genai.Text("hello"), nil)
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("native upstream error was not preserved: %v", err)
	}

	if calls.Load() != 1 {
		t.Fatalf("native request retried %d times", calls.Load()-1)
	}
}
