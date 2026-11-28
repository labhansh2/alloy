package openai_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alloy/node"
	"alloy/provider"
	"alloy/provider/openai"
	"alloy/server"

	native "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

func TestClientUsesNativeResponses(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer upstream-secret" {
			t.Error("client did not use the native route and server-side auth")
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"test-model"`) {
			t.Error("client did not send the selected model")
		}

		if !strings.Contains(string(body), `"store":false`) || !strings.Contains(string(body), `"stream":true`) || strings.Contains(string(body), "max_output_tokens") {
			t.Error("request does not meet the ChatGPT plan Responses contract")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"status\":\"completed\"}}\n\n")
	}))
	defer upstream.Close()

	srv := server.New()

	auth, _ := provider.BearerToken("upstream-secret")
	if err := srv.AddProvider("test", provider.Config{BaseURL: upstream.URL + "/v1", Auth: auth}); err != nil {
		t.Fatal(err)
	}

	token := strings.Repeat("a", 32)
	if err := srv.AddClient("example", server.Client{Token: token, Providers: []string{"test"}}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- srv.Run(ctx)
	}()

	defer func() {
		cancel()

		if err := <-done; err != nil {
			t.Error(err)
		}
	}()

	select {
	case <-srv.Ready():
	case <-ctx.Done():
		t.Fatal("server did not become ready")
	}

	n, err := node.New(node.Config{Name: "example", ServerURL: "http://" + srv.Addr(), ClientToken: token})
	if err != nil {
		t.Fatal(err)
	}

	connection, err := n.Provider(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}

	client := openai.Client(connection)

	stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: "test-model",
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: responses.ResponseInputParam{
			responses.ResponseInputItemParamOfMessage("hello", responses.EasyInputMessageRoleUser),
		}},
		Store: native.Bool(false),
	})
	defer stream.Close()

	completed := false

	for stream.Next() {
		if stream.Current().Type == "response.completed" {
			completed = true
		}
	}

	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}

	if !completed {
		t.Fatal("native SDK did not receive response.completed")
	}
}
