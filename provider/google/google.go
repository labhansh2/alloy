// Package google supplies Gemini Developer API authentication and native Google
// Gen AI SDK configuration. It does not wrap inference or support Vertex AI.
package google

import (
	"context"
	"net/http"

	"alloy/internal/providerauth"
	"alloy/provider"

	"google.golang.org/genai"
)

// Provider registers the Gemini Developer API using a server-side API key.
// GOOGLE_API_KEY takes precedence over GEMINI_API_KEY. Keys are read lazily;
// preflight checks local configuration, not remote entitlement or key validity.
// Browser login, credential persistence, and Vertex AI are not implemented.
func Provider() provider.Config {
	config := providerauth.EnvironmentAPIKey(
		"https://generativelanguage.googleapis.com", "X-Goog-Api-Key",
		"GOOGLE_API_KEY", "GEMINI_API_KEY",
	)

	auth := config.Auth
	config.Auth = func(request *http.Request) error {
		// Gemini also accepts query credentials. Never let caller credentials
		// select upstream authority or billing instead of the server's key.
		query := request.URL.Query()
		query.Del("key")
		query.Del("access_token")
		request.URL.RawQuery = query.Encode()
		request.Header.Del("X-Goog-User-Project")

		return auth(request)
	}

	return config
}

// Client returns the native Gen AI SDK configured for an Alloy connection.
// Only the Alloy token is used locally; upstream Google credentials stay on the
// server. The backend, endpoint, and version are explicit to avoid ambient SDK
// settings. HTTP inference and streaming are supported, not Gemini Live.
func Client(ctx context.Context, connection provider.Connection) (*genai.Client, error) {
	return genai.NewClient(ctx, &genai.ClientConfig{
		Backend:    genai.BackendGeminiAPI,
		APIKey:     "alloy-gateway",
		HTTPClient: connection.HTTPClient(),
		HTTPOptions: genai.HTTPOptions{
			BaseURL:      connection.BaseURL(),
			APIVersion:   "v1beta",
			Headers:      http.Header{"Authorization": {"Bearer " + connection.Token()}},
			RetryOptions: &genai.HTTPRetryOptions{Attempts: genai.Ptr(int32(1))},
		},
	})
}
