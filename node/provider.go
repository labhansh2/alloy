package node

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"alloy/internal/nodesession"
	"alloy/internal/providerwire"
	"alloy/provider"
)

// Provider checks access and returns settings for the provider's native HTTP
// client or SDK. This is an independent authenticated request, not a message on
// the node session; it may be used by an application without calling Run.
// The connection retains this client's grants, not a new per-provider token.
// Every gateway request rechecks access, expiry, and revocation on the server.
func (n *Node) Provider(ctx context.Context, name string) (provider.Connection, error) {
	if err := nodesession.ValidateName(name); err != nil || name == "." || name == ".." {
		return provider.Connection{}, errors.New("invalid provider name")
	}

	if n.token == "" {
		return provider.Connection{}, errors.New("provider access requires an Alloy client token")
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, n.serverURL+providerwire.AccessPath(name), nil)
	if err != nil {
		return provider.Connection{}, errors.New("create provider access request")
	}

	r.Header.Set("Authorization", "Bearer "+n.token)

	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("Alloy provider redirects are not supported")
		},
	}

	response, err := client.Do(r)
	if err != nil {
		return provider.Connection{}, errors.New("provider access request failed")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return provider.Connection{}, fmt.Errorf("provider access rejected (HTTP %d)", response.StatusCode)
	}

	return provider.NewConnection(n.serverURL+providerwire.GatewayPath(name)+"/", n.token), nil
}
