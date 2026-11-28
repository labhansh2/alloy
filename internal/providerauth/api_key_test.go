package providerauth

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnvironmentAPIKeyLifecycle(t *testing.T) {
	t.Setenv("ALLOY_TEST_PRIMARY_KEY", "")
	t.Setenv("ALLOY_TEST_FALLBACK_KEY", "")

	names := []string{"ALLOY_TEST_PRIMARY_KEY", "ALLOY_TEST_FALLBACK_KEY"}
	config := EnvironmentAPIKey("https://example.com", "X-Key", names...)
	names[0] = "SHOULD_NOT_BE_READ"

	if config.Login != nil || config.BaseURL != "https://example.com" {
		t.Fatal("unexpected provider configuration")
	}

	if err := config.CheckAuth(context.Background()); err == nil || !strings.Contains(err.Error(), "ALLOY_TEST_PRIMARY_KEY") {
		t.Fatal("missing key did not give setup guidance")
	}

	request := httptest.NewRequest("GET", "/", nil)
	if err := config.Auth(request); err == nil || request.Header.Get("X-Key") != "" {
		t.Fatal("missing key did not fail closed")
	}

	t.Setenv("ALLOY_TEST_FALLBACK_KEY", "fallback-secret")

	if err := config.CheckAuth(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := config.Auth(request); err != nil || request.Header.Get("X-Key") != "fallback-secret" {
		t.Fatal("fallback key was not applied")
	}

	t.Setenv("ALLOY_TEST_PRIMARY_KEY", "primary-secret")

	if err := config.Auth(request); err != nil || request.Header.Get("X-Key") != "primary-secret" {
		t.Fatal("primary key did not take precedence")
	}

	t.Setenv("ALLOY_TEST_PRIMARY_KEY", "invalid secret")

	if err := config.CheckAuth(context.Background()); err == nil || strings.Contains(err.Error(), "invalid secret") {
		t.Fatal("invalid preferred key fell back or exposed a secret")
	}

	if err := config.Auth(request); err == nil {
		t.Fatal("invalid preferred key did not fail request authentication")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := config.CheckAuth(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("preflight ignored cancellation")
	}
}
