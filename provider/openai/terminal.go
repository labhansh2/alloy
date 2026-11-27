package openai

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/term"
)

func printLogin(message string) {
	fmt.Fprintln(os.Stderr, message)
}

func readAPIKey() (string, error) {
	fmt.Fprint(os.Stderr, "OpenAI API key (hidden; stored using the OS credential store): ")

	if term.IsTerminal(int(os.Stdin.Fd())) {
		value, err := term.ReadPassword(int(os.Stdin.Fd()))

		fmt.Fprintln(os.Stderr)

		if err != nil {
			return "", errors.New("could not read API key")
		}

		return strings.TrimSpace(string(value)), nil
	}

	value, err := bufio.NewReader(io.LimitReader(os.Stdin, 16385)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) || len(value) > 16384 {
		return "", errors.New("could not read a bounded API key from stdin")
	}

	return strings.TrimSpace(value), nil
}

func openBrowser(ctx context.Context, url string) error {
	var command *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "open", url)
	case "windows":
		command = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.CommandContext(ctx, "xdg-open", url)
	}

	return command.Run()
}
