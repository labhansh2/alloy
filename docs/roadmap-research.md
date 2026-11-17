# Roadmap, research and validation

## Research conclusions

These are directional decisions based on the sources below, not measured Alloy performance claims. Revisit against prototypes, operating cost, supported platforms and real workload data.

- **gRPC + protobuf** supplies cross-language unary and streaming RPC, not a job queue or transparent durable delivery. HTTP/2 and its deployment/proxy behavior need practical testing; a Unix socket is a local connection option, not a cross-platform universal solution. Explicit application-level auth, limits, retry and reconnect are mandatory.
- **NATS** separates lightweight Core NATS from JetStream persistence; JetStream consumers provide at-least-once delivery and acknowledgements. It is attractive if Alloy's own queue grows beyond a single host, but broker administration and a dependency are costs. Its deduplication mechanisms do not make arbitrary external side effects exactly once. Evaluate embedded versus external deployment, licensing, upgrades and resource footprint before adoption.
- **SQLite WAL** offers a simple single-host state store, but has one writer at a time and is not a distributed database. Its documentation excludes WAL over a network filesystem. Benchmark queues and crash recovery rather than extrapolating database speed from marketing.
- **MQTT** is designed for lightweight pub/sub; QoS levels address protocol delivery, not the exactly-once execution of an arbitrary handler or external action. Use a gateway only when a concrete embedded device requires it.
- **Workload identity** can use short-lived certificates and mutual TLS, with separate authorization. SPIFFE/SPIRE provide established identity and attestation concepts, but running SPIRE for every personal server would raise setup cost. Do not design custom cryptographic primitives.
- **Runtime management**: uv documents managed Python and project lockfiles. Go's toolchain can build portable binaries for supported targets, but native code and dependencies affect portability. Node package installs can execute lifecycle scripts, a supply-chain and user-consent concern. No runtime manager eliminates offline, platform or native dependency constraints.
- **Secrets**: OWASP recommends lifecycle management, least privilege, audit and rotation; storing values encrypted on a host does not protect them when a compromised server process can decrypt them. Broker operations instead of distributing secrets wherever possible.

## Milestones and exit criteria

Order is deliberately incremental; each milestone produces a demo and tests before its successor. Do not treat speculative interfaces as shipped APIs.

### 0 — Agree on the contract and baseline

- Record target users, first automation, supported OS/architectures, deployment model and trust expectations. Inventory current public API and user changes; add tests around existing typed subscriptions and lifecycle.
- Prototype a minimal Go node and resource use with the proposed manifest; review the UX with one human and one agent. Decide whether durable delivery is required for the first end-user scenario.
- Exit: approved terminology and behavioral examples; documented compatibility plan and initial threat model.

### 1 — Local vertical slice

- CLI initializes/starts/stops a loopback-only server; profile/list/status distinguish configuration from live process. Bind state directory ownership safely. Add manifest registration and explicit policy approval.
- Trusted Go in-process node publishes typed ephemeral events, one logical subscriber with competing workers, resource broker mock, bounded queues and clean shutdown. Preserve current engine adapter while extracting interfaces.
- Exit: reproducible end-to-end demo and tests for fan-out, worker sharing, queue saturation, cancellation, invalid requests, denied permissions and server restart behavior. Explicitly label ephemeral delivery.

### 2 — Durable delivery and recovery

- Define and implement SQLite schema/migrations for events, subscription delivery records, lease/attempt state and dead letters. Atomic publish+fan-out transaction; ack/lease renewal, expiry, retry, quotas, retention and explicit inspection/replay. Design locking and clock assumptions; no replay without authorization.
- Crash-injection tests at commit, delivery, side effect and ack boundaries; fill disk/queue; restart and demonstrate no accepted committed jobs are lost within documented storage assumptions. Verify duplicates, attempt limits and subscriber isolation.
- Exit: semantics documented with observed failure behavior, performance baseline, backup/restore test; no exactly-once claim.

### 3 — Process nodes and language SDKs

- Versioned protobuf/gRPC local transport and protocol negotiation, local identity, Go process SDK, Python/TypeScript SDKs with identical fixtures; initial runner installs/executes only approved projects. Pin runtimes and lockfiles; add Python and TS example nodes.
- Exit: same scenario runs across all three languages, each separately restarted, with capability checks and durable redelivery verified. Test OS-specific transport support and offline cache behavior.

### 4 — Secure remote and resource integrations

- Implement/review enrollment, server identity verification, mTLS, authorization, audit, credential rotation/revocation and reconnect. Add external secret store/keychain integration; built-in vault only after dedicated design review. First useful narrowly scoped integration and approved MCP adapter.
- Exit: two-machine demo, network interruption and reconnect, malicious/expired/replayed credential tests, cross-server isolation, secret leak tests, operational recovery documentation and independent security review before public exposure.

### 5 — Product hardening

- Supervised service installation, platform matrix, structured `--json` CLI, upgrade/migration, resource quotas, observability, docs, examples, backup and restore. Benchmark and publish results, profiling real bottlenecks.
- Exit: documented SLOs based on measured deployment, release and rollback procedure, migration coverage, reproducible install and a real dogfooded workflow.

### Later experiments, gated by demand

Server-hosted resource providers on remote nodes; constrained-device gateway for one selected board; NATS/JetStream or Postgres backend; project hot reload; clustered availability; federation; additional transports. Each requires separate security, semantics and operations decisions. Never prioritize transparent optimization over correctness.

## Test matrix and performance plan

**Correctness**: contract fixtures across Go/Python/TS and transport adapters; version skew and incompatible handshake; per-subscription fan-out and competing replicas; publish with no subscribers; quota behavior; denied publish/subscribe/resource calls; duplicates/idempotency; schema errors; cancellation and drain. Test crash before/after database commit, lease and ack, plus node restart, server restart, partitions, disk full, corrupted input and restore. Fuzz protocol decoders and manifest parsing. Run race detection for Go and concurrent registration/shutdown tests.

**Benchmarks**: on explicitly recorded OS, CPU, disk, language versions and topology, compare in-process channel, same-host Unix socket and remote gRPC at payloads such as 128 B, 4 KiB and 256 KiB; ephemeral and durable separately. Measure p50/p95/p99 end-to-end latency, publish/ack throughput, startup/cold/warm install time, CPU, RSS, disk writes, queue growth and recovery time under slow consumer and network loss. Include serialization and broker costs, warmup, repetitions and concurrency. Use a local durable database with real fsync settings and disclose whether resource calls are mocked. No transport is selected as "fastest" without these measurements and required semantics.

**Operational gates**: upgrades and downgrades/rollback, CA expiry/revocation, credential rotation, restore, least-privilege resource use, node version skew, full queues, unexpected exit and offline behavior. Establish numeric budgets only after milestone 0 defines target hardware and workload.

## Decisions to make with the user

1. What is the first real automation and does it require durable delivery, remote nodes or just local channels?
2. Is the initial audience a single trusted user per server or mutually untrusted users? The proposed design isolates servers, but does not offer hostile multi-tenancy inside one server.
3. Which operating systems/architectures are launch requirements? Is Windows required for v1, and which Raspberry Pi model?
4. Should the default server be purely declarative or is a Go-coded server a core requirement? Keep both possible, avoid promising that a source file is a running server ID.
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
