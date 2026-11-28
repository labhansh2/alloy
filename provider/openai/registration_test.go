package openai_test

import (
	"context"
	"strings"
	"testing"

	"alloy/provider/openai"
	"alloy/server"
)

func TestConfigureOpenAIAndMissingAuthGuidance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")

	srv := server.New()
	if err := srv.AddProvider("openai", openai.Provider()); err != nil {
		t.Fatal(err)
	}

	err := srv.CheckAuth(context.Background())
	if err == nil {
		t.Fatal("missing auth did not fail preflight")
	}

	for _, hint := range []string{"OPENAI_API_KEY", "login", "browser", "token"} {
		if !strings.Contains(err.Error(), hint) {
			t.Errorf("missing setup guidance for %s", hint)
		}
	}

	if strings.Contains(err.Error(), "--") {
		t.Fatal("provider guidance assumes command-line flags owned by the application")
	}

	t.Setenv("OPENAI_API_KEY", "test-upstream-key")

	if err := srv.CheckAuth(context.Background()); err != nil {
		t.Fatal(err)
	}
}
