# Roadmap, research and validation

**Status: scenario-led research plan, not a release commitment.** The [vision](../vision.md) is a growing personal intelligent system, not an automation engine or tooling proxy alone. Server APIs, launch mechanisms, node execution, and transports remain open; [design status](../private/design-status.md) records the decisions to make later.

## Research conclusions

These are candidate technologies and engineering constraints based on earlier research, not selected architecture or measured Alloy performance claims. Revisit against prototypes, operating cost, supported platforms and real workload data.

- **gRPC + protobuf** supplies cross-language unary and streaming RPC, not a job queue or transparent durable delivery. HTTP/2 and its deployment/proxy behavior need practical testing; a Unix socket is a local connection option, not a cross-platform universal solution. Explicit application-level auth, limits, retry and reconnect are mandatory.
- **NATS** separates lightweight Core NATS from JetStream persistence; JetStream consumers provide at-least-once delivery and acknowledgements. It is attractive if Alloy's own queue grows beyond a single host, but broker administration and a dependency are costs. Its deduplication mechanisms do not make arbitrary external side effects exactly once. Evaluate embedded versus external deployment, licensing, upgrades and resource footprint before adoption.
- **SQLite WAL** offers a simple single-host state store, but has one writer at a time and is not a distributed database. Its documentation excludes WAL over a network filesystem. Benchmark queues and crash recovery rather than extrapolating database speed from marketing.
- **MQTT** is designed for lightweight pub/sub; QoS levels address protocol delivery, not the exactly-once execution of an arbitrary handler or external action. Use a gateway only when a concrete embedded device requires it.
- **Workload identity** can use short-lived certificates and mutual TLS, with separate authorization. SPIFFE/SPIRE provide established identity and attestation concepts, but running SPIRE for every personal server would raise setup cost. Do not design custom cryptographic primitives.
- **Runtime management**: uv documents managed Python and project lockfiles. Go's toolchain can build portable binaries for supported targets, but native code and dependencies affect portability. Node package installs can execute lifecycle scripts, a supply-chain and user-consent concern. No runtime manager eliminates offline, platform or native dependency constraints.
- **Secrets**: OWASP recommends lifecycle management, least privilege, audit and rotation; storing values encrypted on a host does not protect them when a compromised server process can decrypt them. Broker operations instead of distributing secrets wherever possible.

## Milestones and exit criteria

First prove the user experience, then select the infrastructure it needs. Each stage produces a demo and tests; later tracks are conditional, not a fixed sequence requiring a durable queue before useful SDK or capability integration. Do not treat speculative interfaces as shipped APIs.

### 0 — Choose a personal-system scenario

- Start with a person building and extending their own intelligent system through an existing agent or handwritten code. Choose a real need and a useful subsequent extension, not only an event-routing benchmark.
- Earlier planning suggested macOS and local deployment first. Local-first is the intended journey; confirm the initial OS/architecture support rather than implying cross-platform support already exists.
- Inventory prototype behavior and existing APIs; identify one capability, the smallest node participation model, trust expectations, and the acceptable consequence of lost events.
- Exit: a concrete example of what the user creates, how it helps, how it can grow, and which decisions are necessary to test it. No final API or transport choice required at this stage.

### 1 — Agent-guided local foundation

- Validate installation guidance and a server-authoring skill with an existing agent. Use the SDK to construct a local server; make the same example understandable to a user writing it manually.
- Define the minimum useful CLI and Alloy MCP operations for this example, with explicit administrative versus ordinary client authority. Do not require a separate UI or full command parity.
- Declare one supported provider/harness/MCP or service capability, its credential source, and the client's approved access. Connect a useful node and inspect its result and failure path.
- Preserve validated prototype primitives. Use bounded events and explicit delivery limitations; avoid new mandatory manifests, runtimes, databases, or transport rewrites unless the scenario requires them.
- Exit: a reproducible local experience, denied-access tests, safe credential handling, observable startup/shutdown, and honest restart/offline behavior. Do not expose the prototype publicly as part of this proof.

### 2 — Demonstrate growth and reuse

- Add a differently shaped node that reuses configured capabilities and exchanges meaningful structured events. Keep native application and harness behavior available.
- Change a preference or need and revise the system without rebuilding its foundation. Show who approved the change, what authority it received, and how to stop/revoke it or restore the previous source/configuration. Do not imply that source rollback undoes external side effects.
- Exit: the second capability requires less repeated setup, the change is inspectable, and the user can understand what executes where. Record observed shortcomings before choosing the next engineering track.

### Conditional track — Execution and language support

- Choose the first managed and independently hosted cases, metadata format, connection protocol, and language release order. Go, Python, and TypeScript sketches are candidates, not a requirement to release all three together.
- Test pinned dependencies, consent to installation/execution, clean shutdown, reconnect behavior, and explicit execution/isolation policy. Native SDK access must not depend on giving the node all host credentials.
- Exit: the selected support matrix passes shared behavioral and authorization fixtures, including failure and cleanup paths. A separately hosted example is required before claiming cross-machine portability.

### Conditional track — Secure remote and resource integrations

- Implement/review enrollment, server identity verification, mTLS, authorization, audit, credential rotation/revocation and reconnect. Add external secret store/keychain integration; built-in vault only after dedicated design review. First useful narrowly scoped integration and approved MCP adapter.
- Exit: two-machine demo, network interruption and reconnect, malicious/expired/replayed credential tests, cross-server isolation, secret leak tests, operational recovery documentation and independent security review before public exposure.

### Conditional track — Durable delivery and recovery

- Enter this track only when selected events must survive restarts or disconnection. Choose persistence after defining acceptance, delivery, ack, retention, retry, and idempotency requirements.
- SQLite is a candidate for a single-host queue, not a product requirement. Test crash boundaries, duplicate side effects, disk/queue exhaustion, subscriber isolation, backup/restore, and authorized replay.
- Exit: documented and tested guarantees within stated storage assumptions. Never claim exactly-once external effects or durable workflow execution from a delivery queue alone.

### Conditional track — Product hardening

- Supervised service installation, platform matrix, structured `--json` CLI, upgrade/migration, resource quotas, observability, docs, examples, backup and restore. Benchmark and publish results, profiling real bottlenecks.
- Exit: documented SLOs based on measured deployment, release and rollback procedure, migration coverage, reproducible install and a real dogfooded workflow.

### Later experiments, gated by demand

Server-hosted resource providers on remote nodes; constrained-device gateway for one selected board; NATS/JetStream or Postgres backend; project hot reload; clustered availability; federation; additional transports. Each requires separate security, semantics and operations decisions. Never prioritize transparent optimization over correctness.

## Test matrix and performance plan

**Correctness**: contract fixtures across Go/Python/TS and transport adapters; version skew and incompatible handshake; per-subscription fan-out and competing replicas; publish with no subscribers; quota behavior; denied publish/subscribe/resource calls; duplicates/idempotency; schema errors; cancellation and drain. Test crash before/after database commit, lease and ack, plus node restart, server restart, partitions, disk full, corrupted input and restore. Fuzz protocol decoders and manifest parsing. Run race detection for Go and concurrent registration/shutdown tests.

**Benchmarks**: on explicitly recorded OS, CPU, disk, language versions and topology, compare in-process channel, same-host Unix socket and remote gRPC at payloads such as 128 B, 4 KiB and 256 KiB; ephemeral and durable separately. Measure p50/p95/p99 end-to-end latency, publish/ack throughput, startup/cold/warm install time, CPU, RSS, disk writes, queue growth and recovery time under slow consumer and network loss. Include serialization and broker costs, warmup, repetitions and concurrency. Use a local durable database with real fsync settings and disclose whether resource calls are mocked. No transport is selected as "fastest" without these measurements and required semantics.

**Operational gates**: upgrades and downgrades/rollback, CA expiry/revocation, credential rotation, restore, least-privilege resource use, node version skew, full queues, unexpected exit and offline behavior. Establish numeric budgets only after milestone 0 defines target hardware and workload.

## Decisions to make with the user

The broader [open-question register](../private/design-status.md#questions-to-decide-later) also covers growth, source versus state, agent authority, SDK/CLI/MCP responsibilities, and memory/workspaces. The questions below narrow the engineering scenarios rather than override that register.

1. What is the first useful personal system and how should it grow? Does the scenario require durable delivery, remote nodes, or just local participation?
2. Is the initial audience a single trusted user per server or mutually untrusted users? The proposed design isolates servers, but does not offer hostile multi-tenancy inside one server.
3. Which operating systems/architectures are launch requirements? Is Windows required for v1, and which Raspberry Pi model?
4. Given SDK-led server construction, what initial language and launch/build experience should be supported? Is a stock declarative server useful later? A source file is not a running server ID.
5. Which first resources matter: GitHub CLI/API, AWS, MCP, a database, or arbitrary commands? Narrow adapters give much safer defaults than general shell access.
6. Who approves node code and resource permissions, especially for agent-authored nodes? How should approved changes be reviewed?
7. Is unattended boot mandatory? Choose acceptable unlock trust and external secret-manager dependencies accordingly.
8. Expected job volumes, latency, payloads, uptime and offline retention? These determine whether SQLite is sufficient and which benchmarks matter.
9. Are server-hosted nodes allowed to invoke host tools, and under which OS identity? Is isolation a future requirement?
10. Which specific Arduino board/network is important, if any? A gateway is a proposal, not a commitment to all boards.

## Sources

Primary documentation consulted during planning; URLs provide starting points for implementation-specific verification.

1. [gRPC core concepts and streaming](https://grpc.io/docs/what-is-grpc/core-concepts/) — service, RPC and streaming model.
2. [NATS JetStream concepts](https://docs.nats.io/concepts/jetstream) and [delivery and acknowledgment](https://docs.nats.io/learn/jetstream/delivery-and-acknowledgment) — persistence, consumers and redelivery.
3. [SQLite write-ahead logging](https://www.sqlite.org/wal.html) — single-host and WAL limitations.
4. [MQTT 5.0 OASIS specification](https://docs.oasis-open.org/mqtt/mqtt/v5.0/mqtt-v5.0.html) — protocol semantics for constrained devices.
5. [SPIFFE concepts](https://spiffe.io/docs/latest/spiffe/concepts/) and [SPIRE concepts](https://spiffe.io/docs/latest/spire-about/spire-concepts/) — workload identity, attestation and trust.
6. [OWASP Secrets Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html) — lifecycle, access control and operational guidance.
7. [uv documentation](https://docs.astral.sh/uv/) and [Python version management](https://docs.astral.sh/uv/guides/install-python/) — Python environments and managed installations.
8. [Go toolchains](https://go.dev/doc/toolchain) and [Go modules reference](https://go.dev/ref/mod) — Go compiler and module management.
9. [Node.js package scripts](https://docs.npmjs.com/cli/using-npm/scripts) — installation lifecycle script implications.
10. [Protocol Buffers language guide](https://protobuf.dev/programming-guides/proto3/) — schema evolution and wire format design.

Research here is an initial engineering survey, not an exhaustive security audit or benchmark. Source availability and current tool/runtime platform support must be rechecked during implementation.
