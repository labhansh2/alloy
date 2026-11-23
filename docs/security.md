# Security and resource model

## Trust boundaries and threat model

Protect one user's server from other users, compromised or overprivileged nodes, network observers, accidental repository leaks and lost node credentials. The server host's administrator, an attacker with server-process access, and trusted in-process Go code can generally read or use server resources: encryption at rest does not protect an unlocked vault from them. A privileged host runner can execute code on its host and must be treated accordingly. This design does not claim to sandbox hostile code, prevent authorized nodes from exfiltrating data they legitimately receive, or protect an already compromised endpoint.

Default bind to loopback/local socket. Remote exposure is an explicit setup action with TLS, authentication, authorization, firewall guidance and visible status. A tunnel does not replace authentication. No administrative access, secret value, connection string or host environment is granted just because a node is registered.

## Principals, enrollment and authorization

See [Authentication and sandboxing](auth-and-sandboxing.md) for the proposed low-friction runner/profile flow and execution modes. An administrator may approve a bounded delegation policy once, allowing an authenticated runner to request server-issued credentials for new nodes within that policy without per-node prompts. This does not remove instance authentication or per-operation authorization. The direct enrollment flow below is one proposed remote setup path, not a requirement to manually enroll every runner-managed node.

Each server has a generated stable instance ID and TLS identity, and its own CA or equivalent trusted identity root. An administrator is bootstrapped locally via restrictive OS permissions; remote admin access must be separately configured. A node definition has an ID; each running installation has a distinct revocable credential and instance ID. A host runner is its own principal, not an implicit administrator.

Proposed remote enrollment:

1. An authenticated admin approves a node definition, its manifest digest, requested grants, and target server. The server creates a cryptographically random, single-use, short-lived enrollment token bound to this registration. Display it once; avoid shell arguments, logs and process listings. Offer an interactive pairing code or protected file/stdin; do not place bearer tokens in URLs.
2. The operator starts the node with the server address and **verified server trust**: provision a pinned CA/public-key fingerprint through a separate trusted channel, or use publicly trusted TLS with a verified DNS name. Do not accept a certificate fingerprint fetched from the same unauthenticated connection as proof. Explain first-use fingerprint verification rather than hiding it.
3. Node generates a private key on its host and sends a CSR over the verified TLS channel with the enrollment token. Server atomically consumes the token and issues a short-lived client certificate bound to the server, node and instance. Store keys in OS-protected storage where available and restrictive files otherwise. CSR/token exchange requires rate limits and audit.
4. Thereafter use mTLS plus server-side policy on every operation. Rotate before expiry, revoke lost instances server-side, and re-enroll after expiration; revocation must take effect for existing sessions through disconnect/recheck, not only at next TLS handshake. Clock skew, CA backup/rotation and disaster recovery need an explicit operational procedure before production.

This is a proposed design requiring security review, not a home-grown cryptographic protocol specification. Use established TLS, X.509 and cryptographic libraries. Consider SPIFFE/SPIRE integration for managed deployments later; do not impose its operational cost on a local single-user install.

Policy defaults to deny and is scoped by server, principal, action and resource/subject, with optional constraints. Examples: `publish:deploy.finished.v1`, `subscribe:deploy.requested.v1`, `resource:deploy-api:deployment.create`, `secret:read:example`, `node:register`. Only the administrator grants privileges; node manifests request them. Subscription, event publishing, payload visibility, resource invocation and administrative operations are separately authorized. Enforce authorization for local, remote and in-process calls; in-process checks are consistency, not a security boundary against malicious Go code. Audit grant changes, registration, credential lifecycle and resource use without recording secrets or payloads by default.

## Resource gateway

Expose narrowly defined operations rather than shipping credentials to every worker:

- **Integration/MCP**: server invokes an approved operation using its credential, applies principal/operation policy, timeouts, rate and response limits, records metadata, and returns the permitted result. MCP tools are external code/data: treat tool descriptions and returned text as untrusted, never as authority to override policy. MCP server startup and network access need administrator approval.
- **CLI**: prefer typed adapters or allowlisted command + argument schema. Run on an explicitly named host with restricted environment, working directory, timeout and output size. `aws`/`gh` credentials and profiles are host-specific; do not offer a generic unrestricted shell as a safe resource. Shell metacharacters must not be interpreted when invoking an argument vector. A command with broad credentials still carries broad authority; policy cannot make it intrinsically narrow.
- **Database**: expose preapproved queries/operations or constrained credentials; apply timeouts and result limits. A raw database URL/SQL executor is a privileged escape hatch. Application data storage and Alloy's own metadata store are separate.
- **Secrets/config**: differentiate nonsecret configuration from secret material. Prefer a server-side operation that uses a secret without returning it. Explicit `secret.read` is high-risk, scoped, audited and discouraged; once returned to a node it cannot be recalled. Never promise that environment variables or redaction are fully safe.
- **Future node-provided resource**: node advertises a namespaced operation schema, server admin approves the provider and caller policy, server brokers calls with timeouts and checks, and provider unavailability is surfaced. A node cannot grant itself access or publish a resource that automatically becomes trusted. Defer to after server-local resources work.

## Vault and key lifecycle

First preference is an existing external secret manager or OS keychain adapter; the server stores references, not values, when possible. For a built-in vault, require authenticated encryption with a random data-encryption key wrapped by an OS keychain key or a key derived from a user-supplied passphrase using a vetted memory-hard KDF and unique salt. Document algorithm/version, nonces and associated data, permissions and threat boundaries; do not invent crypto. Never store the wrapping key beside ciphertext unprotected. At-rest encryption applies to vault values, not automatically to event payloads, SQLite, backups, logs or process memory.

For headless startup, require an explicit unlock source (protected FD, OS secret service, external KMS/manager, or operator entry). Automatic unattended unlock necessarily transfers trust to that source/host; do not imply passphrase protection when the passphrase is stored next to the database. Secrets in argv, git, ordinary logs, crash reports and telemetry are prohibited by default. Backups must include encrypted vault data and separately secured recovery keys; test restoration, rotation, revocation and key loss. Prefer short-lived upstream credentials and record which secret version was used without values. Secure deletion from SSDs, copies and logs cannot be guaranteed.

## Security acceptance gates

- Cross-server identity and resource requests fail; an unknown or expired credential fails closed. Revocation terminates existing sessions; token replay fails even under concurrent enrollment.
- Server identity mismatch, bad TLS, unauthorized subject, operation and administrative calls fail, including through local socket and channel adapters.
- Permission changes prompt review; a changed definition cannot silently retain unapproved broader authority. Logs, errors, traces and CLI history tests contain no seeded secret.
- Malicious MCP content cannot grant permissions; CLI argument validation blocks unauthorized command execution. Demonstrate limits of allowed resource output and host-level compromise.
- External security review and dependency/crypto review are release gates for internet exposure. Document residual risks and incident/recovery procedures.
