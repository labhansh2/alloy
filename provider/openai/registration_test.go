package openai_test

import (
	"context"
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

	if err := srv.CheckAuth(context.Background()); err == nil {
		t.Fatal("missing auth did not fail preflight")
	}

	t.Setenv("OPENAI_API_KEY", "test-upstream-key")

	if err := srv.CheckAuth(context.Background()); err != nil {
		t.Fatal(err)
	}
}
