// Package providerauth implements shared first-party provider authentication
// mechanics. Public extension contracts remain in alloy/provider.
package providerauth

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"alloy/provider"
)

// EnvironmentAPIKey builds lazy, server-side API-key authentication. The first
// nonempty variable wins; an invalid value fails closed instead of falling back.
// Construction has no side effects. Preflight checks syntax, not remote validity.
// Credentials are not persisted and no browser/login flow is installed.
func EnvironmentAPIKey(baseURL, header string, variables ...string) provider.Config {
	// Own the list: request authentication can run concurrently with callers.
	names := append([]string(nil), variables...)

	resolve := func() (provider.Authenticator, error) {
		for _, name := range names {
			if key := os.Getenv(name); key != "" {
				auth, err := provider.APIKey(header, key)
				if err != nil {
					return nil, errors.New("invalid provider API key in " + name)
				}

				return auth, nil
			}
		}

		return nil, errors.New("provider API key required; set " + strings.Join(names, " or ") + " on the server")
	}

	return provider.Config{
		BaseURL: baseURL,
		Auth: func(request *http.Request) error {
			auth, err := resolve()
			if err != nil {
				return err
			}

			return auth(request)
		},
		CheckAuth: func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return err
			}

			_, err := resolve()

			return err
		},
	}
}
