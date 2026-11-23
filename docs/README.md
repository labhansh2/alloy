# Alloy architecture plan

This is a proposed design, not a description of implemented features. It translates the product idea into decisions that can be tested before committing to a large rewrite.

1. [Product and architecture](architecture.md): boundaries, developer experience, jobs, transports, and existing code.
2. [Security and resources](security.md): identities, enrollment, vault, authorization, and threat model.
3. [Runtime and deployment](runtime.md): Go/Python/TypeScript, registration, lifecycle, deployment, and embedded devices.
4. [Delivery roadmap and research](roadmap-research.md): milestones, acceptance tests, benchmarks, alternatives, and primary sources.
5. [Node structure](node-structure.md): handler-based node projects, language sketches, and connection lifecycle.
6. [Server structure](server-structure.md): server projects, Go composition, capabilities, node bindings, and webhook routing.
7. [Authentication and sandboxing](auth-and-sandboxing.md): paired runners, reusable approvals, per-node credentials, execution isolation, and resource enforcement.

The [product vision](vision.md) describes the current direction. The structure guides are proposed developer experiences; older release and transport plans below are not requirements for the current memory-backed prototype.

## North star

One user-operated Alloy server coordinates automations and controlled access to resources. Nodes can run in the server process, on the same machine, or remotely. They publish events and consume subscribed work using an SDK; the CLI makes the common path easy for people and agents. The server is the default trust and policy boundary, not a magical way to make every remote machine equally trusted.

## Proposed first release

Single server, Go control plane, local SQLite state, Go nodes in process and as supervised processes, Python and TypeScript process SDKs, remote outbound connections, explicit durable versus ephemeral delivery, resource operations mediated by the server, and simple enrollment. Ship remote connectivity and durable jobs only after their security and crash recovery tests pass. No clustering, federation, untrusted-code sandbox, arbitrary secret synchronization, or microcontroller SDK in v1.

Examples are illustrative API sketches; names, flags and file formats should be validated through prototypes. Open product questions and decision gates are in the roadmap.
