# What should an Alloy server project look like?

The server is **a small composition root for your personal intelligent system**: configure shared capabilities, approve access, connect nodes, and start the runtime.

In other words:

- The server owns the registry, node connections, event routing, resource authorization, and shared gateway.
- Your server project says what capabilities exist and which nodes may use them.
- Nodes contain application behavior. The server does not need to contain every capability's logic or import every node's implementation.
- Go is the selected initial server SDK language; future language support remains open. Nodes remain ordinary programs in their own languages and environments.

This complements [the node structure](node-structure.md) and [the product vision](vision.md).

## Implemented first slice

The minimal Go SDK now provides instance construction and a blocking run method:

```go
srv := server.New() // import "alloy/server"
err := srv.Run(ctx)
```

Run it with `go run ./cmd/minimal-server`. This needs no Alloy CLI, mux, or protocol setup in user code. `New` has no external side effects; `Run` opens an available loopback port and owns startup and shutdown. Cancel the context to stop. Each instance runs once; create a new instance for another run. Optional `Ready()` and `Addr()` methods expose startup completion and the bound address without requiring log scraping.

This slice has an internal health check and a minimal local-only [node session](node-structure.md#implemented-first-slice-standalone-go-node). Session names are unauthenticated metadata and grant no capabilities. There is no resource registration, event transport, provider connection, client authentication, persistent identity/state, or protocol selection. It does not yet replace the richer prototype server. The example prints its bound URL so a separately launched node can connect.

**All remaining layouts, APIs, configuration fields, and commands below are design sketches, not implemented interfaces.** Start with explicit primitives; do not build a polished SDK just to reproduce these examples.

## Intended authoring experience

Users install Alloy through their existing tools, with an installation skill to guide their agent. An SDK, CLI, and Alloy MCP server support the experience; server-authoring skills help agents create an SDK-based server for the user's needs. Writing the same server by hand is equally valid. Exact installation, API, launch, and supervision mechanisms remain open.

Server setup should support explicit declarations for provider authentication, harness authentication/connections, upstream MCPs, skills, service integrations, and client authentication/authorization. These declarations reference protected credentials rather than embedding secrets in source. The Alloy MCP server used by an authoring agent is not the same concept as an upstream MCP exposed to nodes. Its administrative operations and deployment shape still need design.

There is no first-party Alloy UI commitment in this phase. See the [vision](vision.md) for the product boundaries that take precedence over the API sketches below.

## A small server project

One server project could hold the shared foundation of your personal intelligent system:

```text
my-alloy/
├── go.mod
├── go.sum
├── cmd/
│   └── home/
│       └── main.go          # Configure and run this Alloy server
├── internal/
│   └── setup/
│       ├── server.go        # Assemble capabilities and bindings
│       ├── resources.go     # Custom resource adapters, if needed
│       └── nodes.go         # Node registrations and grants
├── nodes/                  # Optional: local node projects
│   ├── note-tagger/
│   │   ├── alloy.node.yaml
│   │   ├── package.json
│   │   ├── pnpm-lock.yaml
│   │   └── src/main.ts
│   └── folder-watcher/
│       ├── alloy.node.yaml
│       ├── go.mod
│       └── main.go
└── README.md
```

This is a possible **user project**, not a requirement to reorganize Alloy's own implementation repository. A small setup can start with just `go.mod`, `go.sum`, and `main.go`; split configuration into files when it helps readability.

Nodes may live in separate repositories. A browser extension or remote worker does not need to be copied into this repository to connect. A path under `nodes/` is a convenience for local projects, not a path that automatically exists on another machine.

Keep node credentials, upstream login files, private keys, and runtime state outside the repository. Reference protected host configuration rather than embedding secret values in source or manifests.

## A minimal Go entry point

The entry point starts an ordinary Go application with a clear lifetime:

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "example.com/my-alloy/internal/setup"
)

func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}

func run() error {
    ctx, stop := signal.NotifyContext(
        context.Background(), os.Interrupt, syscall.SIGTERM,
    )
    defer stop()

    srv, err := setup.NewServer()
    if err != nil {
        return err
    }

    return srv.Run(ctx)
}
```

Here, `setup` is code in the user's project. The proposed `Run` contract is to bind the listener, start assigned workloads, block until shutdown or a fatal error, and clean up before returning. Startup errors must reach the caller rather than only being logged in a background goroutine.

## What server setup contains

The composition code should read like a description of the environment, not a workflow graph. For example, this is one possible Go API shape:

```go
package setup

import (
    "example.com/alloy/server"
)

func NewServer() (*server.Server, error) {
    srv, err := server.New(server.Config{
        Name:     "home",
        Listen:   "127.0.0.1:8080",
        Delivery: server.Ephemeral,
    })
    if err != nil {
        return nil, err
    }

    if err := registerResources(srv); err != nil {
        return nil, err
    }

    if err := registerNodes(srv); err != nil {
        return nil, err
    }

    if err := registerWebhooks(srv); err != nil {
        return nil, err
    }

    return srv, nil
}
```

The helper functions are normal project code. They register definitions before startup; they should not start unmanaged background tasks or execute repository-supplied node code merely by discovering it. The initial setup is memory-backed: no database or durable queue is implied.

The `server` package above is illustrative. It is not an instruction to create a new public package before the current prototype validates the behavior.

## Shared capabilities are configured once

Resource registration names the capability and its implementation. It does not grant every connected node access to it.

An illustrative registration fragment, assuming `srv` and the adapters have already been constructed:

```go
// Typed, server-executed operations.
srv.RegisterResource("github.issue.read", github.ReadIssue)
srv.RegisterResource("notes.tags.update", notes.UpdateTags)

// Native protocol/SDK connections, not wrappers around every tool or method.
srv.RegisterMCP("research", researchMCP)
srv.RegisterHarness("codex", codexConnection)
```

These registration calls should validate definitions and report failures; error handling is omitted in this fragment only to show the categories. Adapter construction should make the execution host, credential source, timeouts, and other limits explicit.

- **Resource operations:** functions such as `github.issue.read` run on the server using an explicitly configured client or approved CLI adapter. Do not substitute a generic unrestricted shell.
- **MCP connections:** Alloy can expose an approved MCP endpoint backed by an upstream HTTP service or a managed server-local stdio process. Nodes use a native MCP client or their harness's MCP support.
- **Harness connections:** Alloy supplies native SDK connection options where supported. A node may run the full Codex SDK locally; registration does not mean the thread or its filesystem tools execute on the server.
- **Instructions and skills:** optional named configuration bundles can be supplied to nodes with explicit selection, compatibility checks, and native overrides. They are not automatically executable or globally authoritative.

Provider integrations are not interchangeable. The current Codex auth proxy is experimental; registering a connection does not establish that forwarding subscription credentials is supported or permitted. Use a separately verified integration path for each provider.

Node-local services do not belong in the server's resource registry just because they exist. A local filesystem watcher or tool remains part of the node unless it is deliberately exposed as an approved provider in a future design.

### Share access, not upstream secrets

Treat a request such as `openai.options` as a request for an approved capability, not a configuration bag containing provider credentials. Where a supported integration permits it, the node SDK can supply native-compatible connection settings: an Alloy endpoint, a scoped Alloy credential, and nonsecret options. Alloy authorizes requests and authenticates upstream; the native harness and workspace can remain on the node.

YAML, if used, requests capabilities and references configuration; it neither stores secrets nor grants access. Exact declarations, descriptors, and adapters remain undecided. An integration requiring upstream credentials on the node needs an explicit security/product decision, never a silent fallback.

**Proposed first resource:** a harmless named operation such as `preferences.read`. First prove authenticated access, an allowed call, and a denied call; then add provider/harness connectivity. Provider options mix configuration, authentication, native-client compatibility, and execution location, making them a poor first boundary test. This is a proposed sequence, not an implemented resource API.

## Register nodes separately from granting access

The node's manifest says what it requests. Server-side bindings say what is approved and whether Alloy should run it.

For example, a server-side binding could conceptually contain:

```yaml
node: note-tagger
manifest: ./nodes/note-tagger/alloy.node.yaml

execution:
  mode: managed
  host: server
  lifetime: service

grants:
  subscribe: [note.updated]
  publish: [note.tagged]
  resources: [notes.tags.update]
  harnesses: [codex]
```

This YAML illustrates the contents of a binding; it does not require a second configuration language or parser for the first prototype. Go registration can express the same data initially. Do not maintain conflicting Go and YAML sources of truth.

Important distinctions:

- **Registration** approves a definition and binding; it is not installation, startup, or proof that a running instance is authenticated.
- **Managed nodes** are explicitly assigned to Alloy's local runner. The runner uses the node's approved entry point and prepared environment. Registration alone must not install or execute dependencies.
- **Self-managed nodes** start elsewhere and connect with their own approved instance identity. Alloy does not assume control over their host processes.
- **One-shot nodes** run once and report completion; their successful exit is not a failure requiring automatic restart. Schedules can start a new run explicitly.
- **Services and workers** remain running under a configured restart/shutdown policy.
- **Frontends and extensions** can connect independently, but must not contain an administrator token or embedded upstream credentials. Browser clients need an appropriate user/session authorization flow and origin checks.

The server should show registered, running, connected, and ready as distinct states. A shared logical node name is not authentication; grants must be enforced against authenticated instances or sessions. The prototype's self-declared name check is not sufficient for production.

## Event subscriptions are the default wiring

Prefer subscribing to meaningful events over hard-coding a graph of node-to-node destinations:

```text
note.updated
    → note-tagger subscription

note.tagged
    → frontend subscription
    → activity-log subscription
```

The server approves publish/subscribe permissions and routes events to connected subscribers. Nodes can remain independently understandable programs. Direct resource calls still use request/response; functions inside a node do not need to pass through the server.

For the initial in-memory path, say explicitly that events are ephemeral. Server restarts and offline subscribers do not imply replay. Separate logical subscriptions receive their own deliveries; replicas sharing one subscription may compete for work. Do not promise those semantics until the transport and broker implement and test them.

## One gateway, without making every node a web server

The server owns the HTTP listener. It can route different kinds of traffic on the same port:

```text
Alloy server listener
├── authenticated node connections
├── authenticated control/status endpoints
├── approved MCP or harness routes
└── explicitly exposed webhook routes
        → connected node handler
```

Paths are internal routing details, not a requirement for node authors to implement HTTP servers. Sharing a port does not mean sharing authorization rules.

A node registers a logical webhook handler. Its manifest requests that capability; the server binding approves a route and the signature/verification policy. External requests arrive at Alloy, which forwards a bounded request envelope over the node's existing outbound session and returns its response.

For example:

```text
POST /hooks/notes
    → verify source/signature using the raw request bytes
    → forward to note-tagger's approved "notes" handler
    ← return handler response within a deadline
```

- Route ownership and node-instance selection must be explicit; nodes cannot claim arbitrary server paths.
- Synchronous verification challenges need request/response behavior, not only event publication.
- Bound payload size, headers, concurrency, and response time. Surface offline-node failures rather than silently acknowledging unprocessed work.
- If Alloy acknowledges after publishing an in-memory event, that is not a durable receipt or proof of completed processing.
- A loopback listener is not internet-reachable. Public webhooks require deliberate exposure through a reachable deployment or an external tunnel; remote node connections require verified TLS and authentication.

This is webhook forwarding, not a general reverse HTTP tunnel. Exposing an existing node-local frontend or HTTP application is a separate feature with its own routing, streaming, and security requirements. Webhook, MCP, and browser-facing endpoints should not inherit control-plane access merely because they share a listener.

## What belongs outside the server main file?

Keep the entry point focused on assembly and lifetime:

- Node handlers and automation-specific decisions live in node projects.
- Custom resource implementations live in ordinary packages, registered by setup code.
- Provider credentials and node-instance credentials live in protected host storage, not committed configuration.
- Prepared language environments and built artifacts live in explicitly managed locations, not assumed to exist on every host.
- Logs and transient status are runtime concerns; durable state can be designed later when required.

A trusted Go node may be explicitly linked into the server as an optional adapter. That is not dynamic Go source loading, and it shares the server's memory and OS privileges. It should use the same logical event/resource contract without pretending to be an isolated process.

### Future goal: application-controlled SDK logging

Alloy should emit internal server/node events through an optional injected `*slog.Logger`; applications control level, format, destination, or disable logging. Handshake details belong at debug level, connection lifecycle at info, and actionable problems at warn/error without duplicating returned errors. Never log credentials or full payloads by default. Currently, handshake logging uses unconditional global `log.Printf` calls; configurable logging is not implemented yet.

## SDK-led construction, with launch details still open

SDK-based server construction is part of the intended user journey, not merely an advanced escape hatch. A small Go-coded composition root is one candidate for testing the primitives. Users or their coding agents can assemble custom adapters without waiting for a large declarative schema.

A stock server executable loading declarative setup is another possible convenience, not a selected default or an obligation to implement both paths. If offered, it should assemble the same runtime and enforce the same checks. A source file is not a running server identity; how the CLI builds, starts, and identifies SDK-authored servers remains open.

This is an incremental path, not a commitment to implement both entry points immediately. The current prototype commands remain demonstrations rather than the server API shown here.

## Interactive authentication during server setup

A server may integrate with services that require a user to authenticate interactively. One possible setup experience is a login mode on the server executable—for example, a future `go run ./cmd/home --login --browser` command—that opens the relevant provider or service login pages in the user's browser. The exact command-line options and whether the browser is local, remote, or otherwise mediated are undecided.

The intended outcome is that the user completes each required authentication flow during setup rather than configuring every service through a separate manual process. This is not necessarily one authentication across unrelated services: each service may require its own consent, account, or login flow. After successful authorization, the server could retain credentials in protected host storage, refresh tokens when supported, and use service-specific authentication logic when making requests.

This remains a design idea, not an implemented capability or a commitment to a particular OAuth library, credential store, or browser flow. Any implementation must make requested scopes and services visible, keep secrets out of source code and logs, handle revocation and refresh failures, and distinguish user login credentials from node-instance authentication and client access grants. Services that do not support refresh tokens or browser-based authorization will need an explicit alternative, and some credentials may require re-authentication.

## My recommendation

Make the minimum server contract:

1. An explicit listener, instance configuration, and shutdown lifetime.
2. A registry of shared capabilities with known execution locations.
3. Approved node bindings, instance authentication, and scoped grants.
4. Event routing and direct resource request/response.
5. Optional local supervision for explicitly assigned nodes.
6. A shared gateway that can forward approved webhooks without node listening ports.
7. Status and errors that explain what ran, where it ran, and why it failed.

Build one useful personal system with these primitives first. Keep the setup small enough that adding or refining a node mainly means writing its behavior and declaring what it needs—not rebuilding the foundation or inventing another infrastructure stack. Automations can demonstrate this, but do not define the product's scope.
