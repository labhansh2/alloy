// Command minimal-node connects a standalone Go program to a local Alloy server.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"

	"alloy/node"
)

const (
	exampleProvider = "openai"
	exampleModel    = "gpt-6-luna"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := loadDotEnv(); err != nil {
		return err
	}

	serverURL := flag.String("server", envOrDefault("ALLOY_SERVER_URL", ""), "local Alloy server base URL")
	name := flag.String("name", envOrDefault("ALLOY_NODE_NAME", "minimal-node"), "descriptive node name")
	callProvider := flag.Bool("call-provider", false, "call the hardcoded provider and model once, then exit")
	prompt := flag.String("prompt", envOrDefault("ALLOY_PROMPT", "Say hello in one sentence."), "input for the optional provider example")

	flag.Parse()

	n, err := node.New(node.Config{
		Name:        *name,
		ServerURL:   *serverURL,
		ClientToken: os.Getenv("ALLOY_CLIENT_TOKEN"),
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *callProvider {
		return runProviderExample(ctx, n, exampleProvider, exampleModel, *prompt)
	}

	return n.Run(ctx)
}

func loadDotEnv() error {
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}
