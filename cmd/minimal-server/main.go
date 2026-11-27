// Command minimal-server runs an Alloy server using the public Go SDK.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/joho/godotenv"

	"alloy/provider"
	"alloy/provider/openai"
	"alloy/server"
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

	loginDefault, err := envBool("ALLOY_LOGIN")
	if err != nil {
		return err
	}

	login := flag.Bool("login", loginDefault, "authenticate the registered OpenAI provider, then exit")
	auth := flag.String("auth", envOrDefault("ALLOY_AUTH", "browser"), "login method: browser (ChatGPT plan) or token (API key)")

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New()
	if err := srv.AddProvider("openai", openai.Provider()); err != nil {
		return err
	}

	if *login {
		return srv.Login(ctx, provider.LoginOptions{Method: *auth})
	}

	if err := srv.CheckAuth(ctx); err != nil {
		return err
	}

	if err := srv.AddClient("minimal-node", server.Client{
		Token: os.Getenv("ALLOY_CLIENT_TOKEN"), Providers: []string{"openai"},
	}); err != nil {
		return fmt.Errorf("set a random ALLOY_CLIENT_TOKEN (at least 32 characters) for the node: %w", err)
	}

	return srv.Run(ctx)
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

func envBool(key string) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s value %q: %w", key, value, err)
	}

	return parsed, nil
}
