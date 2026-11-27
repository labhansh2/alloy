# Alloy documentation

Start with the [product vision](vision.md). Root-level documents describe architecture, core design, and the public research index; [prototype guides](#prototypes) record experiments and verified findings, not a production feature contract. Private planning documents are intentionally excluded from this public documentation index.

## Architecture and design

1. [Product and architecture](architecture.md): boundaries, developer experience, jobs, transports, and existing code.
2. [Security and resources](security.md): identities, enrollment, vault, authorization, and threat model.
3. [Runtime and deployment](runtime.md): Go/Python/TypeScript, registration, lifecycle, deployment, and embedded devices.
4. [Delivery roadmap and research](research/roadmap-research.md): milestones, acceptance tests, benchmarks, alternatives, and primary sources.
5. [Node structure](node-structure.md): handler-based node projects, language sketches, and connection lifecycle.
6. [Server structure](server-structure.md): server projects, Go composition, capabilities, node bindings, and webhook routing.
7. [Authentication and sandboxing](research/auth-and-sandboxing.md): paired runners, reusable approvals, per-node credentials, execution isolation, and resource enforcement.
8. [Deployment research](research/deployment-research.md): server versus node hosting, existing-system integrations, platform constraints, and optional cloud onboarding.

These are proposed designs. Broader release and transport plans are not requirements for the current memory-backed prototype.

## Prototypes

The first public Go server SDK slice is described in [server structure](server-structure.md#implemented-first-slice); run its standalone example with `go run ./cmd/minimal-server`. It covers server lifetime only, not the capabilities demonstrated by the separate prototypes below.

- [Shared prototype server](prototype/prototype-server.md)
- [Events prototype](prototype/events-prototype.md)
- [Codex prototype](prototype/codex-prototype.md)
- [Browse/MCP prototype](prototype/browse-mcp-prototype.md)

These notes preserve the experiments' original scope and limitations. They are not evidence that the intended SDK, CLI, Alloy MCP interface, skills, or production security model are complete.
