// Command prototype-notion-node is an ordinary MCP client. It knows only an
// Alloy endpoint and node credential, not how Notion's OAuth works.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"alloy/internal/prototype/mcpgateway"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	connectionPath := flag.String("connection", "", "protected Alloy connection JSON file (not an OAuth cache)")
	listOnly := flag.Bool("list", false, "show the approved tools and their actual schemas without searching")
	query := flag.String("query", "alloy-prototype-smoke-check", "read-only Notion search query")

	flag.Parse()

	if *connectionPath == "" {
		return errors.New("provide -connection with the server-generated node connection file")
	}

	data, err := os.ReadFile(*connectionPath)
	if err != nil {
		return errors.New("cannot read node connection file")
	}

	var connection struct {
		Endpoint string `json:"endpoint"`
		Token    string `json:"token"`
	}
	if json.Unmarshal(data, &connection) != nil || connection.Token == "" {
		return errors.New("invalid node connection file")
	}

	u, err := url.Parse(connection.Endpoint)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.Path != "/mcp/notion" || u.User != nil || u.RawQuery != "" {
		return errors.New("prototype expects its fixed loopback MCP endpoint")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "alloy-notion-node", Version: "prototype"}, nil)

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: connection.Endpoint, HTTPClient: mcpgateway.BearerClient(connection.Endpoint, connection.Token), DisableStandaloneSSE: true, MaxRetries: -1,
	}, nil)
	if err != nil {
		return errors.New("cannot connect to Alloy MCP; check that the server and this node credential are current")
	}
	defer session.Close()

	count := 0

	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return errors.New("cannot list tools through Alloy")
		}

		count++

		if tool.Name != "notion-search" {
			return errors.New("server exposed an unexpected tool")
		}

		fmt.Println("Approved tool:", tool.Name)

		if *listOnly {
			schema, _ := json.MarshalIndent(tool.InputSchema, "", "  ")
			fmt.Println(string(schema))
		}
	}

	if count != 1 {
		return fmt.Errorf("expected exactly one approved tool, got %d", count)
	}

	if *listOnly {
		return nil
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "notion-search", Arguments: map[string]any{
		"query": *query, "query_type": "internal", "page_size": 1, "max_highlight_length": 0,
	}})
	if err != nil {
		return errors.New("Notion search request failed through Alloy")
	}

	if result.IsError {
		return errors.New("Notion search returned a tool error; inspect the current tool schema with -list")
	}
	// Avoid dumping workspace contents to chat/logs just to prove access.
	fmt.Printf("Read-only Notion search succeeded through Alloy (%d content blocks; workspace content not printed).\n", len(result.Content))

	return nil
}
