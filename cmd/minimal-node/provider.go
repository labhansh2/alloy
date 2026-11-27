package main

import (
	"context"
	"errors"
	"fmt"

	"alloy/node"
	"alloy/provider/openai"

	native "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

func runProviderExample(ctx context.Context, n *node.Node, name, model, prompt string) error {
	connection, err := n.Provider(ctx, name)
	if err != nil {
		return err
	}

	client := openai.Client(connection) // The native OpenAI SDK, not an Alloy inference API.

	stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: responses.ResponseInputParam{
			responses.ResponseInputItemParamOfMessage(prompt, responses.EasyInputMessageRoleUser),
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
