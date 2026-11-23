package mcpgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGatewayFiltersToolsAndRequiresNodeCredential(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var reads, writes atomic.Int32

	upstreamServer := mcp.NewServer(&mcp.Implementation{Name: "mock-notion", Version: "test"}, nil)
	for _, name := range []string{"notion-search", "notion-create-pages"} {
		upstreamServer.AddTool(&mcp.Tool{Name: name, InputSchema: json.RawMessage(`{"type":"object"}`)}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if req.Params.Name == "notion-search" {
				var args map[string]any
				if json.Unmarshal(req.Params.Arguments, &args) != nil || args["scope"] != "server-pinned" {
					t.Error("server argument constraints were not applied upstream")
				}

				reads.Add(1)
			} else {
				writes.Add(1)
			}

			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "read result"}}}, nil
		})
	}

	upstreamHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return upstreamServer }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})

	upstreamHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer notion-upstream-secret" {
			t.Error("upstream received the wrong credential")
			http.Error(w, "unauthorized", http.StatusUnauthorized)

			return
		}

		upstreamHandler.ServeHTTP(w, r)
	}))
	defer upstreamHTTP.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "gateway-upstream", Version: "test"}, nil)

	upstream, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: upstreamHTTP.URL, HTTPClient: BearerClient(upstreamHTTP.URL, "notion-upstream-secret"), DisableStandaloneSSE: true, MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()

	handler, err := NewTools(ctx, upstream, Options{
		Token: "node-only", AllowedTools: []string{"notion-search"},
		FilterCall: func(_ string, raw json.RawMessage) (json.RawMessage, error) {
			var args map[string]any
			if json.Unmarshal(raw, &args) != nil || args["query"] != "test" {
				return nil, fmt.Errorf("denied")
			}

			args["scope"] = "server-pinned"

			return json.Marshal(args)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	host := httptest.NewServer(handler)
	defer host.Close()

	for _, token := range []string{"", "wrong", "notion-upstream-secret"} {
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, host.URL, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}

		r.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}

		resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("unexpected status %d", resp.StatusCode)
		}
	}

	nodeClient := mcp.NewClient(&mcp.Implementation{Name: "node", Version: "test"}, nil)

	node, err := nodeClient.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: host.URL, HTTPClient: BearerClient(host.URL, "node-only"), DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()

	tools, err := node.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(tools.Tools) != 1 || tools.Tools[0].Name != "notion-search" {
		t.Fatalf("unexpected tools: %+v", tools.Tools)
	}

	result, err := node.CallTool(ctx, &mcp.CallToolParams{Name: "notion-search", Arguments: map[string]any{"query": "test"}})
	if err != nil || result.IsError {
		t.Fatalf("read failed: %v, %+v", err, result)
	}

	denied, err := node.CallTool(ctx, &mcp.CallToolParams{Name: "notion-search", Arguments: map[string]any{"query": "blocked"}})
	if err != nil || !denied.IsError {
		t.Fatalf("server policy rejection failed: %v, %+v", err, denied)
	}

	_, err = node.CallTool(ctx, &mcp.CallToolParams{Name: "notion-create-pages", Arguments: map[string]any{}})
	if err == nil {
		t.Error("unlisted write tool was not denied")
	}

	if reads.Load() != 1 || writes.Load() != 0 {
		t.Errorf("unexpected upstream calls: reads=%d writes=%d", reads.Load(), writes.Load())
	}

	if _, err := NewTools(ctx, upstream, Options{Token: "node-only", AllowedTools: []string{"missing"}}); err == nil {
		t.Error("missing approved tool should fail startup")
	}
}

func TestBearerClientDoesNotSendTokenElsewhere(t *testing.T) {
	client := BearerClient("http://127.0.0.1:1/mcp", "private")
	if _, err := client.Get("http://127.0.0.1:2/mcp"); err == nil {
		t.Fatal("credential must not be sent to another endpoint")
	}
}
