package main

import (
	"crypto/rand"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"alloy/internal/prototype/codexproxy"
	"alloy/internal/prototype/mcpoauth"
)

func registerCodex(mux *http.ServeMux, baseURL, stateDir, browserToken string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	codeHome := os.Getenv("CODEX_HOME")
	if codeHome == "" {
		codeHome = filepath.Join(home, ".codex")
	}

	token := rand.Text()

	tokens := []string{token}
	if browserToken != "" {
		tokens = append(tokens, browserToken)
	}

	proxy, err := codexproxy.New(filepath.Join(codeHome, "auth.json"), tokens...)
	if err != nil {
		return err
	}

	mux.Handle("/codex/", proxy)

	path := filepath.Join(stateDir, "codex-node-connection.json")
	if err := mcpoauth.SavePrivateJSON(path, struct {
		BaseURL string `json:"baseUrl"`
		Token   string `json:"token"`
	}{baseURL + "/codex", token}); err != nil {
		return err
	}

	log.Printf("Codex ready: %s/codex; node connection: %s", baseURL, path)

	return nil
}
