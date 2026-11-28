package provider

import (
	"errors"
	"net/http"
	"strings"
)

// APIKey applies a server-owned key in the given HTTP header. Use BearerToken
// for Authorization: Bearer credentials instead. The key must be nonempty
// printable ASCII without whitespace. Header names must be valid HTTP tokens.
// It neither obtains nor persists credentials, and does not implement login.
func APIKey(header, key string) (Authenticator, error) {
	if header == "" {
		return nil, errors.New("provider API key header must be nonempty")
	}

	for _, c := range header {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", c)) {
			return nil, errors.New("invalid provider API key header")
		}
	}

	if key == "" {
		return nil, errors.New("provider API key must be nonempty")
	}

	for _, c := range key {
		if c < 33 || c > 126 {
			return nil, errors.New("invalid provider API key")
		}
	}

	return func(request *http.Request) error {
		request.Header.Set(header, key)
		return nil
	}, nil
}
