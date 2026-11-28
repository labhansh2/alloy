// Command server runs the minimal Alloy SDK example.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"alloy/provider"
	"alloy/provider/openai"
	"alloy/server"

	"github.com/joho/godotenv"
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

	login := flag.Bool("login", false, "log in to OpenAI in the browser, then exit")

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New()
	if err := srv.AddProvider("openai", openai.Provider()); err != nil {
		return err
	}

	if *login {
		return srv.Login(ctx, provider.LoginOptions{Method: "browser"})
	}

	if err := srv.AddClient("minimal-node", server.Client{
		Token:     os.Getenv("ALLOY_CLIENT_TOKEN"),
		Providers: []string{"openai"},
	}); err != nil {
		return err
	}

	return srv.Run(ctx)
}
