# Product vision

Agents are great at writing code solutions and automations that improve personal productivity. Writing that code is increasingly accessible; connecting it to tools, giving it the right resources, deploying it, and keeping it running still involves repetitive work.

Alloy exists to remove that friction:

> Write the automation. Alloy provides its operating environment and connects it to your intelligence and tools.

## Personal automation, powered by your intelligence

Alloy is a personal automation runtime backed by a central collection of your intelligence tooling, integrations, and configuration. It connects your automations to those capabilities and to each other.

The starting point is useful personal automation, not a generic workflow engine or a universal credential proxy. Intelligence portability supports that goal rather than replacing it.

Ordinary code handles predictable work. Agent or model capabilities handle ambiguous, nondeterministic tasks within that same program. Authors should not have to rebuild authentication, service wiring, webhook hosting, scheduling, or process management for every automation.

## Nodes are ordinary programs

A node can be as simple or as complex as its task requires:

- A routine that runs once and exits.
- A scheduled task.
- A long-running watcher or worker.
- A substantial application using a full native agent SDK, such as Codex.
- A frontend or browser extension connected to Alloy.
- A standalone agent using harness SDKs.

A node is not necessarily one workflow step. It may contain an entire automation, many handlers, internal state, and background work. Normal functions, modules, and dependencies remain useful; not every function needs to become a node or shared resource.

The common contract is **communication and capability access**, not a mandatory class structure, language, hosting model, or deployment lifecycle. Alloy may start and supervise a worker, while a frontend or browser extension manages its own lifecycle and connects independently. Nodes need not all be running at the same time.

The goal is to remove repeated setup and infrastructure work, not ordinary code organization or imports.

## A central collection of capabilities

Configure and discover capabilities in one place, then make approved access available to nodes:

- Harnesses, agent SDK connections, and inference providers.
- Integrations, MCP tools, and approved CLI operations.
- Shared instructions, skills, and configuration.
- Reusable automation capabilities.

Nodes declare what they need; Alloy supplies approved access without granting every node all available authority. Registration alone must not expose all credentials, tools, or host files.

Complex nodes should retain the full native SDK where needed. Alloy can supply connection options and authentication brokering through supported integration paths rather than replacing an SDK with a restrictive `askAgent()` abstraction. Native SDK dependencies still belong to the node's environment; they are not live objects shared across processes.

"Central" means a common place to configure, discover, and authorize capabilities, not that all code or data must execute or live on the server. It is not a promise of universal shared conversation memory or session migration.

## Execution location is explicit

- **Server-exposed resources** execute on the server under its configured identity and dependencies. Nodes invoke approved operations without needing the underlying credentials.
- **Node-local services** execute in the node's environment, alongside its files, tools, and runtime. They may work independently or use server resources when authorized.

A node can compose both without moving its entire automation onto the server. Each capability has its own execution location, permissions, dependencies, and availability. A disconnected node cannot use a server resource merely because it was configured previously.

Authentication support is provider-specific. A successful technical experiment does not establish a supported or permitted integration. Alloy must respect provider restrictions and must not promise that every consumer subscription can be shared across arbitrary workloads.

## Events connect nodes

An emitter/subscriber architecture lets independently written nodes work together. Nodes publish events and subscribe to the events relevant to their work.

For example:

```text
Browser extension emits: page.saved
    → Enrichment worker uses Codex
    → Worker emits: page.enriched
        → Notes integration saves the result
        → Frontend updates its view
```

Not everything is an event. Resource operations also need direct request/response interactions, and functions inside a node remain normal function calls. Use events where decoupling helps rather than forcing every operation into a distributed workflow.

Delivery and failure behavior must be explicit. Event-based communication alone does not imply durable delivery, replay, retries, or exactly-once execution. One-shot and intermittently connected nodes need lifecycle and delivery behavior appropriate to their use case.

## What Alloy should make easier

- Configure shared connections once rather than repeat service setup in every automation.
- Use reusable trigger primitives for webhooks, schedules, polling, and events.
- Start and supervise managed nodes without requiring each author to reinvent deployment plumbing.
- Connect independently hosted nodes without assuming Alloy owns their lifecycle.
- Understand what is running, what configuration loaded, where an operation executes, and why it failed.
- Compose simple scripts and sophisticated intelligent programs without imposing a new programming model on all their internal code.

Deployment cannot erase real differences between environments. Alloy should manage or explain the necessary setup, not pretend that dependencies, files, identities, or network access are identical everywhere.

## Build toward the vision incrementally

Keep the useful work on node connections, shared resources, execution locality, and native SDK options. These are building blocks for the automation experience, not separate product directions.

First, make one real personal automation useful end to end: configure its services, write/register a node, run it, trigger it, and inspect the result or failure. Then add a differently shaped node to test whether the primitives generalize.

Start with one managed execution environment and simple configuration. Do not require a database, durable workflow engine, universal configuration translator, polished Alloy SDK, or support for every node type to prove the core experience. Those are later decisions driven by actual automations.

The practical test is:

> Does the second automation require substantially less plumbing than the first, while still allowing the author to write the program they need?

This document describes the intended direction, not a claim that these capabilities are already implemented. Detailed architecture and prototype notes should be evaluated against this vision.
