// Package anthropic supplies Anthropic API-key authentication and native SDK
// configuration. It does not wrap inference or reuse Claude Code credentials.
package anthropic

import (
	"alloy/internal/providerauth"
	"alloy/provider"

	native "github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// Provider registers the Anthropic API using ANTHROPIC_API_KEY on the server.
// Construction reads no credentials. Preflight checks local configuration, not
// remote entitlement or key validity. Browser/Claude subscription login and
// credential persistence are not implemented.
func Provider() provider.Config {
	return providerauth.EnvironmentAPIKey(
		"https://api.anthropic.com", "X-Api-Key", "ANTHROPIC_API_KEY",
	)
}

// Client returns the native Anthropic SDK configured for an Alloy connection.
// Environment credentials, profiles, endpoint overrides, and custom headers are
// not inherited. The SDK sends only Alloy authority to the gateway and supplies
// its normal API-version header. The server injects the upstream API key.
func Client(connection provider.Connection) native.Client {
	return native.NewClient(
		option.WithoutEnvironmentDefaults(),
		option.WithBaseURL(connection.BaseURL()),
		option.WithAuthToken(connection.Token()),
		option.WithHTTPClient(connection.HTTPClient()),
		option.WithMaxRetries(0),
	)
}
