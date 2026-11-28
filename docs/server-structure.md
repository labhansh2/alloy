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

Run the OpenAI example with `go run ./examples/alpha/server` after the authentication setup below. No Alloy CLI, mux, or protocol setup is needed in user code. `New` has no external side effects; `Run` checks managed provider auth before opening an available loopback port, reports its URL, and owns shutdown. It never starts browser login implicitly. Cancel the context to stop. Each instance runs once, even on startup failure; create another instance to retry. Optional `Ready()` and `Addr()` expose startup completion and the bound address.

The server also supports local [node sessions](node-structure.md#implemented-first-slice-standalone-go-node) and the provider connections below. Anonymous sessions remain handshake-only; names alone grant no capabilities. There is no event transport, administrative endpoint, persistent identity/state, or protocol selection. It does not yet replace the richer prototype server. The example prints its bound URL so a separately launched node can connect.

## Implemented provider connections

Register OpenAI in one call; its integration owns endpoint and credential setup. Client grants remain explicit and separate:

```go
if err := srv.AddProvider("openai", openai.Provider()); err != nil {
    return err
}

if err := srv.AddClient("minimal-node", server.Client{
    Token:     os.Getenv("ALLOY_CLIENT_TOKEN"),
    Providers: []string{"openai"},
}); err != nil {
    return err
}
```

The imports are `alloy/provider/openai`, `alloy/server`, and `os`. On the node, `connection, err := n.Provider(ctx, "openai")` checks access and `client := openai.Client(connection)` configures the **native OpenAI SDK**. Use `client.Responses` or other supported native APIs directly—no Alloy inference wrapper, hand-written HTTP transport, goroutine, or idle node session is required for a single request. Generic clients can still use `BaseURL()`, `Token()`, and `HTTPClient()`.

The returned token is the client's Alloy credential with its existing grants—not the upstream key or a newly narrowed per-provider token. Every gateway request checks grants, expiry, and revocation independently of the WebSocket session. Node names remain descriptive; authenticated client identity comes from the token.

Other HTTP services can still use `provider.Config{BaseURL: ..., Auth: ...}`. `provider.Authenticator` is a concurrent-safe request callback and `provider.BearerToken` supplies static bearer auth. Optional `CheckAuth` and `Login` callbacks let integrations own setup; `Run` performs auth preflight, while explicit `srv.Login(ctx, provider.LoginOptions{Method: ...})` invokes login before `Run`. These hooks do not normalize provider inference APIs.

Providers require HTTPS, except explicitly registered loopback HTTP services. The gateway preserves native bodies, query parameters, status/error responses, and HTTP streaming; it replaces caller authentication with server-side auth and strips ambient cookies, forwarding identity, and common authentication headers. Service-specific header/account-selection constraints belong in the registered authenticator, not a provider catalog in Alloy. Redirects, protocol upgrades, and traversal-style paths are unsupported. Defaults bound request bodies to 1 MiB and total request duration to one minute; `MaxRequestBytes` and `Timeout` customize these limits.

**Authority and lifecycle:** a grant covers all supported HTTP operations beneath the configured upstream base URL, not just inference or a selected model. No model filtering, spending/concurrency quotas, automatic retries, or full provider-protocol compatibility is promised. `AddProvider` / `AddClient` are pre-start configuration; `RevokeClient` denies subsequent admissions but does not cancel already admitted work or disconnect an otherwise idle node session. `Client.ExpiresAt` is optional; zero lasts until revocation/process exit. Use distinct random client tokens of at least 32 printable non-whitespace ASCII characters, never names or upstream API keys. Tokens are hashed in the server registry; upstream auth remains in server memory. Local transport still lacks verified server identity and protection against same-user credential theft.

### Gemini and Anthropic API keys

Register `google.Provider()` from `alloy/provider/google` for the Gemini Developer API, or `anthropic.Provider()` from `alloy/provider/anthropic` for the Anthropic API. Google reads server-side `GOOGLE_API_KEY` then `GEMINI_API_KEY`; Anthropic reads `ANTHROPIC_API_KEY`. Construction is lazy, and startup checks local key presence/syntax rather than remote validity. Neither integration implements browser login or managed credential persistence. Their node helpers configure the native Google Gen AI and Anthropic SDKs with Alloy authority, not upstream secrets or ambient SDK credentials. See [provider integrations](../provider/README.md) for registration, native calls, package boundaries, and custom providers.

### OpenAI login and the minimal example

The alpha server registers OpenAI and reuses existing managed credentials. Its only flag is `-login`, which performs explicit login and exits:

```sh
go run ./examples/alpha/server -login
```

The login call hardcodes method `browser` for eligible ChatGPT-plan access. Change its method to `token` for hidden API-key entry. Login saves credentials and exits. The browser flow prints an authorization link and attempts to open it on the server machine, using a temporary `127.0.0.1` callback. It uses OpenAI's documented dynamic client registration, PKCE, state/nonce validation, signed OIDC identity checks, and granted plan-usage scopes—not copied Codex credentials. Availability depends on app/account eligibility under OpenAI's local/open-source preview; a hosted commercial gateway is not implied.

Saved credentials are encrypted atomically using AES-GCM beneath the OS user configuration directory (`alloy/providers/openai/`); the encryption key lives in the OS credential store. There is no plaintext fallback if that store is unavailable. Refreshes are serialized across processes and persist rotating tokens together. Encryption, locking, and atomic writes use the shared credential-storage engine while OpenAI retains its existing paths, keyring entry, authenticated data, credential schema, and registration host identity. Custom integrations can use the public [`credentials` store](../credentials/README.md); adopting its new explicit namespaces does not automatically migrate or isolate legacy OpenAI profiles per server. Saved account credentials take precedence over `OPENAI_API_KEY`; that server-only environment variable remains available when no login is saved. Use `openai.WithAccount("work")` to keep another registration separate; an existing ChatGPT registration cannot silently switch identity or be replaced by token login. Automatic account picking, logout/revocation UI, and backup/migration of these records are not implemented.

The alpha examples read `ALLOY_CLIENT_TOKEN` from the environment, separately from upstream credentials. Copy their `.env.example` templates to ignored `.env` files and configure the same protected random token of at least 32 non-whitespace ASCII characters on both sides. Do not commit real tokens. Without the login flag, run these in separate terminals:

```sh
go run ./examples/alpha/server
# Replace PORT with the printed server port.
go run ./examples/alpha/node -server http://127.0.0.1:PORT
```

The node hardcodes provider `openai`, model `gpt-6-luna`, and its prompt directly in `examples/alpha/node/main.go`; the selected model must be available to the active account. It always makes one native SDK streaming Responses call and exits after `response.completed`; interrupted, failed, and incomplete streams are errors. It sends array input and `store: false`, omitting `max_output_tokens` to match the ChatGPT-plan preview contract. API-key usage and ChatGPT plan usage have different billing/limits. The gateway does not rewrite requests to meet these constraints; native callers must follow the selected authentication mode's contract.

The only command-line options are server `-login` and node `-server`. Both programs load an optional `.env` from the working directory without overriding exported variables. The node also accepts `ALLOY_SERVER_URL` as the default for `-server`; provider, model, prompt, and client name remain hardcoded. The OpenAI integration supports its server-only `OPENAI_API_KEY` fallback. The node needs no upstream credentials. See the [alpha README](../examples/alpha/README.md) for the minimal workflow.

Automated tests cover mock OAuth callbacks, signed identity validation, scope/account rejection, refresh rotation, encrypted storage, and mock inference. A manual minimal-server/node run also returned successful inference using existing server credentials; the active authentication method was not inspected, so this does not establish live browser-login validation or universal account eligibility. Official references checked 2026-11-27: [registration/sign-in](https://developers.openai.com/siwc/token-sharing-open-source/sign-in), [refresh and accounts](https://developers.openai.com/siwc/token-sharing-open-source/profiles-and-sessions), and [preview request constraints](https://developers.openai.com/siwc/token-sharing-open-source/preview-limitations).

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

The configuration APIs in this sketch are illustrative. Only the instance/lifecycle API and `AddProvider` / `AddClient` registrations documented above are implemented.

## Shared capabilities are configured once

Resource registration names the capability and its implementation. It does not grant every connected node access to it.

### Registry and native service connections

The intended server registry covers provider/service connections, tools, MCP gateways, skills, and client authentication/policy. Future entries may include HTTPS tunnels, remote workspaces, approved server directories, and tunneled directory access. These share discovery and authorization concepts, not necessarily a wire protocol or execution mechanism. Registration is not permission, and a directory reference does not copy, mount, or relocate its contents.

For provider inference, keep the boundary thin: **register native service connections with pluggable server-side authentication**, rather than maintain another model router or universal inference API. A named connection identifies an approved upstream endpoint/protocol and account/auth reference. Authorized nodes receive native-compatible Alloy endpoint settings and scoped Alloy credentials; upstream credentials remain on the server. Preserve native SDKs, requests, responses, and supported streaming instead of translating model catalogs or normalizing every provider feature.

New models or upstream API features should generally not require Alloy changes when the native protocol remains compatible. Specialized adapters are still needed where authentication, protocol behavior, or native-client configuration differs. Endpoint overrides are not universally supported. Fine-grained restrictions require protocol-aware enforcement; a transparent connection must not imply model/tool/operation filtering it cannot enforce. See [the security boundary](security.md#native-connection-boundaries).

Separate three concerns without prescribing packages or interfaces yet:

- **Protocol/service adapter:** handles the upstream connection and compatibility requirements, without wrapping every model or operation.
- **Named account connection:** selects the account and protected authentication source, including an API key or a supported browser/account flow.
- **Access grants:** determine which clients can use that connection and any enforceable constraints.

For example, an OpenAI API connection and a supported Codex account connection may belong to the same vendor but have different protocols, authentication, billing, and permitted capabilities. They are not interchangeable credentials.

### Harness options compose approved capabilities

A provider connection supplies inference access. A harness integration supplies native execution/session access or native-compatible configuration. Harness options can compose references to approved provider connections, MCP tools, skills/instructions, and a workspace rather than duplicate their authentication setup.

Additional harness settings may cover local-process versus remote-service execution, workspace/filesystem access, session creation/resumption, native binary/SDK compatibility, tool approvals/sandboxing, and cancellation/time limits. Retain native options and APIs; do not flatten these into a universal `agent.run()` or lowest-common-denominator schema. Exact composition APIs remain open.

A provider-agnostic harness can reuse a registered provider connection only when its protocol, endpoint configuration, authentication path, and permitted usage match. A remote harness service may also require its own login, separate from the provider it uses for inference. Referencing a remote workspace or server directory does not make it node-local or move harness execution there.

Later placement/path optimization may reduce server-to-node distance and unnecessary hops. It must preserve grants, credential boundaries, execution location, and required protocol semantics—not silently relocate work or bypass authorization. There is no automatic routing/placement optimization in the current SDK.

### One connection from an existing tool

The intended experience is one explicit connection from an existing tool such as OpenCode, Codex, or Claude Code: authenticate the instance, discover its approved capabilities and available nodes, and configure supported native provider/MCP connections. MCP access alone does not configure inference providers, skills, or sandbox policy. Keep configuration/availability changes discoverable; access to newly added capabilities requires an explicit delegation policy, not automatic administrator authority.

Direct connections should not require a local Alloy server. An optional future Alloy helper on a user's primary machine could link to a selected remote server, expose approved local interfaces (such as MCP, a credential helper, or a provider-specific proxy), and prepare native configuration. This can make approved capabilities feel local without making upstream secrets local. The helper is a distinct device principal, not a universal administrator; broad user authorization still requires explicit server grants and visible local approval for sensitive actions. It must not silently import logins, expose host resources, or modify tool settings.

For `gh`, `aws`, or another CLI, distinguish an Alloy-mediated operation on an approved execution host from a local shell command. A local helper can broker explicitly supported operations so the CLI/agent need not receive the long-lived upstream secret. If native CLI compatibility requires a local credential, the helper may instead provision a short-lived, narrowly scoped credential; the credential is then available to that CLI and must be treated as exposed to its process/user. Do not imply both direct native authentication and zero credential transfer. Show the account, execution host, and file-access boundary, and request reauthentication when necessary rather than silently changing accounts or placement.

### Human devices and one-purpose workloads

The intended trust model distinguishes a user's primary machine from a purpose-bound job. A primary-machine helper may be authorized for a broad collection of user-approved capabilities, while each node/process still receives only its explicit grants. A Lambda, CI job, or one-purpose server node should instead authenticate as its own workload and receive only the task's grants. Prefer platform workload identity or OIDC exchange for short-lived credentials; a unique revocable, narrowly scoped, expiring key is a fallback, never a shared human credential. This is a design direction: the current SDK uses operator-provisioned bearer tokens and implements neither device helper nor workload-identity exchange. See [security principals and vault design](security.md#human-authorized-devices-and-purpose-bound-workloads).

### Illustrative registration categories

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

**Validation before provider access:** prove authenticated access, an allowed call, and a denied call. A harmless operation such as `preferences.read` can test that boundary; it is not a required first product feature. A provider-backed example should then exercise a native service connection, not establish a new Alloy-specific inference wrapper. No resource API or final implementation sequence is selected yet.

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

OpenAI implements explicit login through `srv.Login`; the alpha server selects it with `-login`, as described above. Generalizing this experience to other services, remote browsers, and mediated callbacks remains future work; each integration must use its own supported flow.

The intended outcome is that the user completes each required authentication flow during setup rather than configuring every service through a separate manual process. This is not necessarily one authentication across unrelated services: each service may require its own consent, account, or login flow. After successful authorization, the server could retain credentials in protected host storage, refresh tokens when supported, and use service-specific authentication logic when making requests.

Treat supported browser/account login as a first-class authentication option alongside API keys. Authentication setup should be explicit; normal startup should reuse configured credentials or report that reauthentication is needed, not unexpectedly launch browsers. OAuth authorization does not itself grant inference entitlements: verify the allowed clients, endpoints, scopes, and subscription-backed usage for each integration. A ChatGPT/Codex login is not automatically a general OpenAI API credential. Our experimental proxy does not establish provider permission or a stable supported integration.

The OpenAI integration does not establish universal OAuth compatibility or a general credential vault. Every additional integration must make requested scopes and services visible, keep secrets out of source/logs, handle revocation and refresh failures, and distinguish upstream credentials from Alloy client grants. Services without refresh tokens or browser authorization need an explicit alternative; some credentials require reauthentication.

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
