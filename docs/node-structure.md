# What should a node look like in each language?

A node is **an ordinary, reusable, connectable program**. A registered set of handlers with an optional lifecycle is a useful authoring style, not the definition of every node. Preserve the plain-function examples below while leaving exact APIs, required metadata, and execution modes open.

**Status: a minimal Go connection/lifecycle SDK is implemented; the remaining authoring patterns are proposals.** Existing applications, watchers, connectors, agents, and user-built frontends should be able to participate without turning every internal function into an Alloy handler. The [vision](vision.md) takes precedence where earlier sketches prescribe a mandatory manifest, runner, or SDK-owned main loop.

## Implemented first slice: standalone Go node

```go
import "alloy/node"

n, err := node.New(node.Config{
    Name:      "deployment-watcher",
    ServerURL: "http://127.0.0.1:8080", // Use the running server's address.
    // ClientToken: protectedAlloyToken, // Optional; required for provider access.
})
if err != nil {
    return err
}

return n.Run(ctx)
```

`New` validates configuration without connecting or installing anything. `Run(ctx)` makes an outbound connection, completes a versioned handshake, and waits until cancellation or disconnection. Canceling the caller's context stops normally; connection/handshake failures and server disconnection return errors. Each instance runs once, with no automatic reconnect. `Ready()` closes after a successful handshake, remains closed after disconnection, and never closes on startup failure; callers waiting on it must also observe the run result or their context.

The current session transport is an internal WebSocket connection, not part of the public authoring API. Nodes do not open a listening port. This slice supports only HTTP URLs with a literal loopback IP and explicit port; it rejects remote hosts, redirects, proxies from environment configuration, and browser-origin sessions. An optional `ClientToken` authenticates the connection; without it, the session is anonymous and handshake-only. A supplied name is descriptive metadata, not identity. Handshake success does not prove server identity or grant provider access. There are no events, application handlers, or durable node registration yet; remote connections require a separate secure design.

### Use a registered provider

With a server-approved `ClientToken`, `n.Provider(ctx, "openai")` checks access and returns native client settings: `connection.BaseURL()` and `connection.Token()`. Feed these into a compatible native SDK or HTTP client; the upstream key stays on the server. This access check is a separate authenticated HTTP request and does not require `Run`. Every subsequent gateway request rechecks authorization, so a previously obtained connection does not bypass revocation. The settings retain the client's existing grants, rather than minting a narrower token.

For OpenAI, `client := openai.Client(connection)` from `alloy/provider/openai` returns the native SDK configured with the Alloy connection. A one-shot request needs neither `n.Run` nor lifecycle/transport goroutines. See [provider setup and login](server-structure.md#openai-login-and-the-minimal-example). The connection itself has no background lifecycle; callers own native requests, streams, and cleanup. Anonymous nodes cannot obtain provider access. Browser/OAuth login happens only on the server; upstream credentials never need to be copied to the node.

Run the examples in separate terminals:

```sh
go run ./examples/alpha/server
# Use the URL printed by the server, replacing PORT below.
go run ./examples/alpha/node -server http://127.0.0.1:PORT
```

The alpha server registers OpenAI, so complete its login setup first. Both alpha examples read `ALLOY_CLIENT_TOKEN` from their environment; use their `.env.example` templates to configure the same protected random token on each side. The alpha node makes one inference call and exits; the long-lived session API above remains available to other programs. A bare `server.New()` in other programs still needs no provider or account.

### One node concept, optional declarative metadata

The SDK node is the running participant. A future YAML manifest would describe that same participant, not create a second kind of node or execute on its own. It could supply requested capabilities, subscriptions, and execution metadata to the SDK or a managed runner. Code supplies behavior and handlers; declarations request access, while server policy grants it. The loading API and how code/YAML avoid conflicting declarations remain open. **The current SDK is code-first and requires no YAML.**

### Dependencies and execution ownership

For now, users prepare dependencies and launch each node with their normal tools: Go modules/builds, npm or pnpm, uv or a manually managed virtual environment, as appropriate. Only the Go node SDK exists today; other language examples below are proposals. Alloy supplies connectivity, not implicit package installation, shared virtual environments, or inherited server credentials. Node and server placement on the same machine does not change that boundary. Deliberately embedded Go nodes would instead share the server's build dependencies and privileges.

Alloy-managed entry points, environment preparation, and supervision remain long-term goals, not requirements for a self-managed node. A future runner should install or execute only explicitly approved projects.

## Longer-term authoring patterns

For a standalone handler-based node, one possible design is:

- The user writes ordinary functions and registers them with the Alloy SDK.
- A small `main` entry point starts the node runtime.
- The running node connects to the server through the Alloy protocol; the server does **not** instantiate a Python class or TypeScript object directly.
- For trusted Go code compiled into the server, Alloy can provide an optional in-process adapter using the same handler contract.

This gives each language an idiomatic API without making Alloy’s architecture depend on one language’s object model.

### What a node looks like

A node should be a small project rather than necessarily a single file:

```text
my-node/
├── alloy.node.yaml
├── main.py               # Or main.go / src/index.ts
├── ...                   # Normal project files
└── dependency lockfile
```

In this candidate layout, the manifest tells Alloy **how to start the node and what it requests**. The code tells the SDK **what to do**. Whether this metadata must be a manifest, can be declared in code, or is needed for independently managed applications remains open. Metadata requests access; it never grants it.

```yaml
apiVersion: alloy.dev/v1alpha1
kind: Node
name: deployment-watcher
runtime: python
entrypoint: main.py

subscriptions:
  - deployment.requested.v1

publishes:
  - deployment.completed.v1

resources:
  - name: github
    operations: [deployment.read]
```

The manifest should not contain server credentials or hard-code a server. A node can be registered with different servers, subject to each server’s approval.

Built-in trigger declarations could cover webhooks, schedules, tickers, and polling, building on the legacy direction. For example, a webhook declaration could request a logical route and reference protected service credentials; secrets must not be embedded in YAML. Route approval, signature verification, dispatch behavior, and exact trigger schemas remain future design work.

### Go

The implemented Go entry point is shown above. The following handler API is a **future sketch**, not the current `alloy/node` API. Use a `main` package for a standalone node, with normal functions as handlers:

```go
package main

import (
    "context"

    "example.com/alloy/sdk"
)

type DeploymentRequested struct {
    ID string `json:"id"`
}

func main() {
    node := alloy.NewNode("deployment-watcher")

    alloy.On(node, "deployment.requested.v1",
        func(ctx context.Context, job alloy.Job[DeploymentRequested]) error {
            // Perform work using approved resources.

            return node.Publish(ctx, "deployment.completed.v1", map[string]any{
                "id": job.Data.ID,
            })
        },
    )

    node.Run() // Connects, receives work, handles shutdown.
}
```

The exact Go API needs prototyping, especially around typed handlers and resource access. The important point is that users can write a plain function; they don’t need to implement a large interface.

For embedding, expose a constructor that registers the same handlers:

```go
func Register(node *alloy.Node) {
    alloy.On(node, "deployment.requested.v1", handleDeployment)
}
```

A Go server that imports this package could then call `Register`. This only works when the node is compiled into the server; it is **not** a way to dynamically load arbitrary Go source. It also runs with the server process’s privileges.

### Python

Use functions and an optional decorator, with a conventional entry point:

```python
from alloy import Node, Context, Job

node = Node("deployment-watcher")


@node.on("deployment.requested.v1")
async def handle_deployment(ctx: Context, job: Job):
    await ctx.publish(
        "deployment.completed.v1",
        {"id": job.data["id"]},
    )


if __name__ == "__main__":
    node.run()
```

In the future managed-execution mode, Alloy would run the project’s entry point as a separate process. Self-managed users run it themselves. Users can organize implementation into classes if they want, but Alloy shouldn’t require a class for a simple handler.

### TypeScript

Use function registration and an explicit `run` call:

```ts
import { Node } from "@alloy/sdk";

const node = new Node("deployment-watcher");

node.on("deployment.requested.v1", async (ctx, job) => {
  await ctx.publish("deployment.completed.v1", {
    id: job.data.id,
  });
});

await node.run();
```

In the future managed-execution mode, Alloy would run the compiled JavaScript entry point in its own process; self-managed applications retain their own launch/lifecycle. The project can use classes internally without making classes part of the required Alloy contract.

### Why not have the server instantiate a class?

It sounds convenient, but it creates different problems for every language:

- A Go server cannot directly instantiate Python or TypeScript classes in the way it instantiates Go types.
- Embedding multiple language interpreters complicates dependencies, failures, upgrades, memory use and portability.
- A broken node could affect the server process; remote nodes couldn’t use the same mechanism anyway.
- Dynamic Go plugins have significant portability and compatibility limitations.

A **process + protocol** boundary works locally and remotely and gives the server a consistent way to supervise, reconnect and communicate. The tradeoff is process and serialization overhead—so the optional embedded Go path is useful when that overhead matters and the code is trusted.

### What should the node “endpoint” actually be?

I would **not** require each node to start its own HTTP server or listen on a port.

Instead, the node SDK initiates a connection to the Alloy server and holds a session open:

```text
Node SDK ── connects ──► Alloy server

Server ── job over session ──► Node handler
Server ◄── ack / result ───── Node SDK
Server ◄── publish event ───── Node SDK
Server ◄── resource request ── Node SDK
```

That works well for remote machines behind NAT and avoids assigning and securing a port per node. The node’s logical endpoint is its **registered handlers and capabilities**, not a network address.

The longer-term SDK should take care of connection authentication, supported reconnect behavior, dispatch, and acknowledgements. These are not implemented by the first handshake-only slice. If retries and durable work are introduced, define the subscription policy explicitly and acknowledge success only after the handler returns successfully; handlers still need to tolerate retries and duplicates. An open session alone supplies no delivery or recovery guarantee.

### Lifecycle without forcing classes

Some nodes need to initialize a client, start a watcher, or close resources. Support optional hooks alongside handlers:

```python
@node.on_start
async def start(ctx):
    # Initialize local state or start a watcher.
    pass


@node.on_stop
async def stop(ctx):
    # Clean up.
    pass
```

A long-running watcher can publish events from its background task; a worker can simply register handlers. For more complex nodes, users can wrap state and handlers in a class—but that remains an application choice.

### Candidate standalone-node contract

For the handler-based authoring style, evaluate:

1. Optional declarative metadata describing how to run it and what it requests; YAML is one proposed format, not a separate node model.
2. An executable entry point that starts the SDK.
3. Zero or more event handlers and optional lifecycle hooks.
4. An SDK-managed outbound connection to the server.
5. Explicit capability approval by the server.

Embedded Go registration remains an optional trusted integration, not the foundational node model or an isolation boundary. A program with its own lifecycle may instead use Alloy only where it needs events or capabilities. The exact process, connection, and managed execution mechanisms will be decided later; the existing channel path is one experiment, not a universal node requirement.
