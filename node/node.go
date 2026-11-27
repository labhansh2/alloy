// Package node connects ordinary Go programs to an Alloy server. The application
// owns its dependencies and process; the node owns its outbound Alloy session.
package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"alloy/internal/nodesession"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// ErrAlreadyRun means Run has already been called on this instance.
var ErrAlreadyRun = errors.New("alloy node instance has already run")

// Config describes a node and its explicit server binding. It contains no
// upstream credentials, grants, dependency installation, or launch instructions.
type Config struct {
	// Name is descriptive metadata, not proof of identity or a unique registration.
	Name string
	// ServerURL is an HTTP base URL with a literal loopback IP and explicit port,
	// for example http://127.0.0.1:8080. Remote access is not supported yet.
	ServerURL string
}

// Node is one outbound session. Construct it with New; do not copy it.
// Its methods are safe to call concurrently. Each instance runs at most once.
type Node struct {
	name     string
	endpoint string
	ready    chan struct{}
	mu       sync.Mutex
	started  bool
}

// New validates configuration without starting processes or making connections.
func New(config Config) (*Node, error) {
	if err := nodesession.ValidateName(config.Name); err != nil {
		return nil, err
	}

	u, err := url.Parse(config.ServerURL)
	if err != nil {
		return nil, errors.New("invalid Alloy server URL")
	}

	ip := net.ParseIP(u.Hostname())

	port, portErr := strconv.Atoi(u.Port())
	if u.Scheme != "http" || ip == nil || !ip.IsLoopback() ||
		portErr != nil || port < 1 || port > 65535 || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("server URL must be an HTTP loopback IP with an explicit port and no credentials, path, query, or fragment")
	}

	u.Path = nodesession.Path

	return &Node{name: config.Name, endpoint: u.String(), ready: make(chan struct{})}, nil
}

// Ready closes after the server accepts the handshake, not after authentication
// or resource authorization. It remains closed after disconnection. If startup
// fails it stays open; callers must also observe Run's result or their context.
func (n *Node) Ready() <-chan struct{} {
	return n.ready
}

// Run connects and blocks until cancellation or disconnection. Caller
// cancellation is a normal stop; connection, handshake, and disconnect failures
// return errors. Run releases its connection before returning and never retries
// or reconnects automatically. A fresh Node is required for another run.
func (n *Node) Run(ctx context.Context) error {
	n.mu.Lock()

	if n.started {
		n.mu.Unlock()
		return ErrAlreadyRun
	}

	n.started = true
	n.mu.Unlock()

	if ctx.Err() != nil {
		return nil
	}

	if err := n.run(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}

		return err
	}

	return nil
}

func (n *Node) run(ctx context.Context) error {
	// Do not route local connections through environment-configured proxies or
	// follow redirects to a host outside the explicitly selected server.
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("Alloy node redirects are not supported")
		},
	}

	handshakeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(handshakeCtx, n.endpoint, &websocket.DialOptions{HTTPClient: client})
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}

		return fmt.Errorf("connect Alloy node: %w", err)
	}
	defer conn.CloseNow()

	conn.SetReadLimit(nodesession.MaxBytes)

	hello := nodesession.Message{Type: "hello", Version: nodesession.Version, Name: n.name}
	if err := wsjson.Write(handshakeCtx, conn, hello); err != nil {
		return fmt.Errorf("send node handshake: %w", err)
	}

	log.Printf("Alloy node handshake: sent hello (name=%q version=%d)", n.name, nodesession.Version)

	var welcome nodesession.Message
	if err := wsjson.Read(handshakeCtx, conn, &welcome); err != nil {
		return fmt.Errorf("receive node handshake: %w", err)
	}

	if welcome.Type != "welcome" || welcome.Version != nodesession.Version {
		return errors.New("unsupported Alloy server handshake")
	}

	log.Printf("Alloy node handshake: received welcome (name=%q version=%d; unauthenticated)", n.name, welcome.Version)

	cancel()
	close(n.ready)

	if _, _, err := conn.Read(ctx); err != nil {
		return fmt.Errorf("Alloy node disconnected: %w", err)
	}

	return errors.New("unexpected Alloy server message")
}
