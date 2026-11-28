// Command node makes one native OpenAI request through an Alloy server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"alloy/node"
	"alloy/provider/openai"

	"github.com/joho/godotenv"
	native "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return err
	}

	serverURL := flag.String("server", os.Getenv("ALLOY_SERVER_URL"), "URL printed by the Alloy server")

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	n, err := node.New(node.Config{
		Name:        "minimal-node",
		ServerURL:   *serverURL,
		ClientToken: os.Getenv("ALLOY_CLIENT_TOKEN"),
	})
	if err != nil {
		return err
	}

	return callModel(ctx, n)
}

func callModel(ctx context.Context, n *node.Node) error {
	connection, err := n.Provider(ctx, "openai")
	if err != nil {
		return err
	}

	client := openai.Client(connection) // The native OpenAI SDK.

	stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: "gpt-6-luna",
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: responses.ResponseInputParam{
			responses.ResponseInputItemParamOfMessage("Say hello in one sentence.", responses.EasyInputMessageRoleUser),
		}},
		Store: native.Bool(false),
	})
	defer stream.Close()

	completed := false

	for stream.Next() {
		event := stream.Current()
		switch event.Type {
		case "response.output_text.delta":
			fmt.Print(event.Delta)
		case "response.completed":
			completed = true
		case "response.failed", "response.incomplete", "error":
			return errors.New("OpenAI response did not complete successfully")
		}
	}

	if err := stream.Err(); err != nil {
		return err
	}

	if !completed {
		return errors.New("OpenAI stream ended before response.completed")
	}

	fmt.Println()

	return nil
}
