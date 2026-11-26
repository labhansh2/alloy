# What should a node look like in each language?

A node is **an ordinary, reusable, connectable program**. A registered set of handlers with an optional lifecycle is a useful authoring style, not the definition of every node. Preserve the plain-function examples below while leaving exact APIs, required metadata, and execution modes open.

**Status: proposed authoring patterns, not finalized interfaces.** Existing applications, watchers, connectors, agents, and user-built frontends can participate without turning every internal function into an Alloy handler. The [vision](vision.md) and [design status](design-status.md) take precedence where earlier sketches prescribe a mandatory manifest, runner, or SDK-owned main loop.

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

### Go

Use a `main` package for a standalone node, with normal functions as handlers:

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

Alloy runs the project’s entry point as a separate process. Users can organize implementation into classes if they want, but Alloy shouldn’t require a class for a simple handler.

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

Again, Alloy runs the compiled JavaScript entry point in its own process. The project can use classes internally without making classes part of the required Alloy contract.

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

The SDK should take care of the connection, authentication, reconnects, dispatch and acknowledgements. On a handler error, Alloy applies the subscription’s retry policy. For durable work, the SDK acknowledges success only after the handler returns successfully; handlers still need to tolerate retries and duplicates.

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

1. A manifest describing how to run it and what it requests.
2. An executable entry point that starts the SDK.
3. Zero or more event handlers and optional lifecycle hooks.
4. An SDK-managed outbound connection to the server.
5. Explicit capability approval by the server.

Embedded Go registration remains an optional trusted integration, not the foundational node model or an isolation boundary. A program with its own lifecycle may instead use Alloy only where it needs events or capabilities. The exact process, connection, and managed execution mechanisms will be decided later; the existing channel path is one experiment, not a universal node requirement.
