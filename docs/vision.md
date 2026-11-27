# Product vision

> Alloy is an engine for building a self-owned, distributed intelligent system that grows with you.

Alloy gives users and their agents a place to turn evolving needs, preferences, and understanding into lasting capabilities. Those capabilities can take the form of ordinary programs, connected tools, skills, configuration, stored knowledge, or agents—not only conversations and not only automations.

This is the intended product direction, not a claim that the current prototype implements it. Engineering details remain proposals until validated by prototypes.

## What “grows with you” means

Your intelligent system should not need to be rebuilt for every new task. As you and your agents discover useful patterns, you can add or refine capabilities that remain available for future work.

Learning does not have to live only in text, vectors, or graphs. It can also become executable software: a watcher that notices something important, a connector to a service, a reusable research capability, or a program that applies a preference you have established.

Growth means being able to extend, connect, and revise this system over time. It does **not** imply automatic model training, unrestricted self-modification, universal conversation memory, or an autonomous agent that decides what your system should become. How changes are proposed, approved, evaluated, and undone remains to be designed. Users decide how much initiative to delegate.

Automations are one useful expression of this intelligence. They are not the product's defining identity. Likewise, portable tooling is an enabling capability, not the whole purpose of Alloy.

## Start through the interfaces people already use

Alloy should complement existing agents and development tools rather than introduce a new primary interface.

The intended experience is:

1. **Install Alloy**, guided by an installation skill that an existing agent can follow. Installation provides an SDK, a CLI, and an Alloy MCP server; packaging details remain open.
2. **Describe the system you need to your agent**, or write the code yourself. Server-authoring skills help agents use Alloy effectively without making agent-generated code mandatory.
3. **Create a server using the SDK.** The server project declares the capabilities and access boundaries the user needs: provider and harness authentication, upstream MCP connections, skills, integrations, and client authentication. Declaring a connection must not mean embedding credentials in source.
4. **Bring up the local server.** The exact start command, build process, supervision, and configuration API are later decisions.
5. **Add and evolve nodes** as new needs emerge. Reuse the system's approved capabilities rather than recreate integration and deployment plumbing each time.

The SDK supports building the server and connected programs. The CLI provides command-line interaction; the Alloy MCP server makes supported Alloy operations available to existing agents. Their exact operation sets, overlap, and authorization are not decided yet. Skills guide usage; they do not grant authority.

An **Alloy MCP server** is different from an **upstream MCP connection** configured as a capability. One lets an agent interact with Alloy; the other gives authorized participants access to tools provided elsewhere. Whether they share a process or listener is not part of this vision.

We are not building a first-party web, TUI, mobile, or desktop interface yet. Users may still build their own interfaces or connect existing ones as nodes; that is different from Alloy shipping its own product UI.

## A server as the system's shared foundation

A [server project](server-structure.md) brings together the tooling, configuration, resources, and access policy for an Alloy instance. Start with one local server as the common coordination point, then let connected programs extend the system across environments.

The server provides a place to declare and make approved access available to:

- Inference providers, including supported local and cloud endpoints.
- Harnesses and native agent SDK connections.
- Integrations, upstream MCP servers, and explicitly approved host tools.
- Instructions, skills, and reusable configuration.
- Storage or memory resources where the user chooses to configure them.
- Client identities and permissions for access to the system.

Register native service connections with server-managed authentication rather than build a universal model router. Provider inference, MCP tools, and harness execution remain distinct capabilities. Harness configurations can compose approved connections, skills, and workspace access while retaining native SDKs and options. Supported browser/account authentication and API keys should both fit; their entitlements and compatibility remain integration-specific.

Remote workspaces, approved server directories, HTTPS tunnels, and tunneled directory access are future registry possibilities, not current features. Common discovery and authorization do not require one protocol for every capability. Later placement/path optimization must preserve authority and explicit execution location.

“Central” means a common place to configure, discover, and authorize capabilities. It does not require every program, credential, or piece of knowledge to live or execute on one machine. One server is the starting model, not a settled design for federation, replication, or a user's entire lifetime of intelligence.

Authentication is provider-specific. A technical proxy experiment does not establish that every consumer subscription can be shared or that a provider permits the intended use. Preserve native SDK capabilities through supported integrations rather than replace every harness with a restrictive universal agent API.

## Nodes are ordinary, connectable programs

Keep the [existing node direction](node-structure.md): reusable, connectable units of execution, written in the language and structure appropriate to their job. A node can be:

- A routine that runs once or on a schedule.
- A long-running watcher, worker, or service connector.
- A substantial program using a full native harness SDK.
- An agent or a program that uses no model at all.
- An existing application, a user-built frontend, or a browser extension connected to Alloy.

A node is not necessarily one workflow step. It can have internal state, ordinary functions and dependencies, background work, and its own application lifecycle. Handler registration and lifecycle hooks remain useful SDK conveniences; their exact APIs and execution arrangements are not fixed by the vision.

Nodes can publish structured events and subscribe to relevant events through the server's broker. Not everything must be an event: resource operations can use request/response, and functions inside a node remain normal function calls. A connector node observes or acts on a service; a resource exposes a capability. Not every configured service needs a node wrapper.

Alloy-managed programs and independently hosted programs should both fit the model. Which execution modes are implemented first, how nodes start, and what delivery guarantees they receive are open decisions. An event broker alone promises neither durable delivery nor workflow recovery.

## Reading the Teralith example

In the supplied sketch, **Teralith** is an example user's Alloy server—not a built-in agent or a required system layout:

- Notion and editor watchers publish changes into the system.
- A command watcher subscribes to relevant changes and emits recognized commands.
- A “dreamer” node receives activity events, uses approved harness configuration and a chosen workspace, and emits its results back into the system.
- Shared setup keeps each node from having to reinvent harness login, MCP access, service configuration, and resource discovery.

The important idea is a collection of useful, independently understandable capabilities that can grow together. The dreamer's behavior, `DREAM.md`, event names, and storage layout are application choices, not Alloy requirements. The sketch does not decide whether the dreamer may modify the system or where its workspace lives.

## Boundaries that should survive implementation choices

- **User-owned and inspectable:** users should be able to understand and change their system rather than depend on an opaque hosted intelligence. Export, backup, migration, and change-history mechanisms still need design.
- **Explicit authority:** registration or agent-written code does not grant every credential, tool, or file. Separate provider authentication, client identity, resource permissions, and host execution access.
- **Explicit execution location:** server resources execute where configured; node-local tools execute in the node's environment. A remote node does not inherit a laptop's browser, filesystem, or login. A disconnected node cannot use an unavailable server resource.
- **Complement existing ecosystems:** retain native application and harness APIs. Do not require replacing workflow frameworks, agent interfaces, or ordinary code organization.
- **Agnostic by direction, specific in support:** language, runtime, OS, provider, and harness independence are goals. Each supported combination needs an actual integration and tests; not all combinations work on day one.
- **Efficient and secure by evidence:** low overhead and secure information exchange are goals, not unmeasured fastest-transport or absolute-security claims.

## How to test the direction

Start with a small personal system: an agent helps create a local server, configures one useful capability, and connects a node that does real work. Add a second node that reuses that setup and exchanges meaningful information with the first.

Then change a need or preference and see whether the system can be adapted without rebuilding its foundation. Inspect the resulting behavior, effective permissions, failures, and how the change can be stopped or undone.

The practical test is:

> Does the system become more useful as you and your agents extend it, while keeping setup, authority, and ongoing maintenance understandable?

Use this test to choose implementation work. Do not make a durable workflow engine, cloud deployment platform, polished UI, or universal configuration translator a prerequisite unless a concrete need demonstrates why it is necessary.
