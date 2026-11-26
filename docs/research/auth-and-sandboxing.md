# Authentication, authorization, and sandboxing

Alloy should make a growing personal intelligent system easy to extend without granting every connected program access to the user's entire environment.

The core idea is:

> Approve a runner and a capability profile once. Automatically provision individual nodes within that boundary, while checking access on every operation.

This is a proposed design, not implemented security functionality. It complements [the security model](security.md), [node structure](node-structure.md), and [server structure](server-structure.md). Configuration examples are illustrative, not a finalized schema or protocol.

Runner profiles and container execution below are candidate mechanisms, not a decision about how all nodes must run. SDK, CLI, and Alloy MCP access also need explicit client roles and permissions; agent-facing skills are guidance, not authorization. See [design status](../private/design-status.md).

## Three separate questions

- **Authentication:** who is making this request? A node name supplied by the caller is not proof of identity.
- **Authorization:** what may that authenticated instance do? The server enforces this automatically on resource calls, subscriptions, publications, and gateway requests.
- **Human approval:** when does the operator need to make a decision? Prefer approving reusable boundaries rather than prompting for every call or newly created node.

Sandboxing answers another question: **what may the program do directly to its execution environment?** Denying a resource through Alloy does not protect a credential file that the node can read directly from the host.

## Pair a trusted runner once

A runner is a process explicitly authorized to start or attach nodes under an approved policy. It has its own identity and is not an implicit server administrator.

The initial experience could be:

1. Install/start a runner on an operator-controlled machine.
2. Pair it with a specific Alloy server, verifying server identity.
3. Approve a capability profile and the local execution policy it may use.
4. Write a node and launch it through that runner.
5. Receive a distinct, short-lived node credential without another login ceremony.

Machine authorization alone is insufficient. Arbitrary programs on that machine must not gain Alloy access merely by claiming its name, address, or a node ID. The runner's credential and trusted launch path establish authority.

The runner's local launch/attachment interface also needs authorization. Restrict it using OS identity/socket permissions and, where applicable, approved projects or explicit launch approval. An unauthenticated local endpoint that issues node credentials would defeat this model.

## Reusable capability profiles

A profile describes a bounded set of capabilities the operator is willing to delegate to nodes launched under it:

```yaml
profile: personal-automation

allows:
  harnesses: [codex]
  resources: [github.issue.read]
  publish: [notes.updated, notes.tagged]
  subscribe: [notes.updated]
```

A node requests the subset it needs. Conceptually:

```text
effective grants = node requests
                ∩ runner/profile authority
                ∩ server policy and constraints
```

The profile cannot grant a capability the runner is not authorized to delegate, and a node manifest cannot grant itself authority. Constraints such as account, repository, model, execution host, or resource limits remain part of authorization, not just the operation name.

Capabilities outside the profile should produce a clear denial or approval request before use. The operator may approve a narrow node-specific exception or deliberately update the reusable profile. Never silently broaden the runner's authority.

Example: new read-only note automations should run without repetitive prompts; requesting GitHub writes, another account, or privileged host access should cross an explicit approval boundary.

For personal development, an operator may deliberately choose a permissive profile. Make the consequence visible: all code launched under that profile is trusted with those capabilities. Broad selectors also need defined behavior when new resources are added; avoid accidentally granting future capabilities through an unclear wildcard rule.

Profiles are reusable approval, not proof that new code is safe. An approved automatic-launch scope may intentionally permit changed code; higher-trust workflows can require reviewed or pinned artifacts. Record which policy allowed each launch rather than treating a changed definition as automatically reviewed.

## Provision node credentials automatically

Prefer having the server issue node credentials after an authenticated runner requests a launch session. The runner does not need a server signing key or permission to invent grants.

The credential/session should bind to:

- The issuing server and intended audience.
- A distinct node instance and its parent runner.
- Server-approved grants and constraints.
- An expiry and a revocable launch/session identity.

The server may retain grants in session state rather than encode them in a bearer token. The exact mechanism can be chosen during implementation; use established authentication libraries rather than designing custom cryptography.

The runner passes only the node's credential and connection configuration through a protected runtime mechanism. Do not pass its own runner credential, server signing keys, or upstream provider tokens to the node. Avoid credentials in command arguments, URLs, committed files, and logs. A protected descriptor or narrowly mounted runtime file is preferable where supported; environment variables are convenient but not a security boundary.

Authenticate the connection once, then authorize every operation against current effective policy without requiring a human prompt each time. Short-lived credentials alone do not provide immediate revocation: disconnect or recheck existing sessions when a node, runner, or profile loses authority. Renewal must re-evaluate policy rather than preserve stale privileges indefinitely.

Runner revocation should invalidate its delegated node sessions. Node revocation should not require revoking unrelated nodes. Existing actions may already have completed; revocation is not rollback.

Disconnected or independently launched programs need an explicit attachment/enrollment flow. Frontends and extensions use an appropriate user/session flow; never embed a long-lived runner or administrator secret in their distributed code.

## Keep host access separate from Alloy access

A node needs two independently approved boundaries:

1. **Execution policy:** filesystem mounts, writable directories, network destinations, OS privileges, and resource limits.
2. **Alloy grants:** which server capabilities, accounts, events, and operations it may use.

A node requests execution requirements in its manifest. The operator's binding approves their actual realization. Editing a manifest must not allow a node to mount the host home directory, access the Docker socket, or increase its resource grants by itself.

### Trusted local process

Appropriate for reviewed personal scripts, desktop integrations, or watchers that intentionally need host access.

- Runs as an explicitly selected OS user with that user's direct host access.
- Alloy still checks resource access, but cannot isolate files or processes accessible to that user.
- A fresh HOME and minimal environment reduce accidental credential/config inheritance; they do not stop code from accessing the real home directory by an absolute path.

Call this trusted execution, not sandboxed execution. Trusted in-process Go nodes are even more privileged: they share server memory and are trusted as server code.

### Isolated managed node

Start with one container execution path instead of custom sandbox technology. The intended baseline is:

- Non-root execution, no privileged mode, reduced OS capabilities, and appropriate host/container security profiles.
- Mount only the intended workspace and necessary runtime files; prefer read-only mounts and explicit writable locations.
- No host home, credential directories, SSH-agent sockets, Docker socket, or runner credentials.
- CPU, memory, process, output, and execution-time limits appropriate to the workload.
- Explicit network policy, ideally limited to Alloy and approved destinations where practical.
- A distinct node credential and scoped server capabilities.

Network restrictions require actual enforcement, including access to host services and metadata endpoints where relevant. A label such as `alloy-only` is not enough. If a requested isolation policy cannot be enforced, fail or require an explicit policy change; do not silently fall back to an unrestricted host process.

Containers are a useful boundary, not a guarantee against hostile code. The container runtime/runner remains privileged infrastructure. Hostile multi-tenant execution is outside the initial scope and may require stronger isolation.

The native harness's own tool sandbox is a separate layer. Running Codex inside a container should not silently disable its protections, and a harness tool sandbox does not automatically sandbox the surrounding node program.

### Independently hosted node

Appropriate for self-managed remote workers, frontends, and browser extensions.

Alloy authenticates and authorizes their connections but does not claim to control their host sandbox. The environment owner supplies host isolation. Browser permissions, session security, and origin checks need their own treatment.

## Resource gateways remain security boundaries

Keeping credentials on the server prevents direct copying, but a credential-free node can still abuse an authorized operation or exfiltrate data returned by it.

- CLI adapters need fixed/approved executables, validated arguments, explicit working directories/environments, timeouts, and output limits. A generic server shell is not a narrowly scoped resource.
- MCP access needs approved tools and operations, not automatic trust in everything an upstream server advertises. Tool descriptions and results cannot grant permissions.
- Harness/model connections need account/model restrictions where supported, usage limits, and provider-compliant authentication paths. An Alloy token authorizes real usage even though it is not an upstream credential.
- Public webhook routes need their own request verification and limits. They must not inherit control-plane authority just because they share the listener.

Authorization must cover every exposed path: native HTTP/provider routes, MCP endpoints, direct resource calls, and event operations. Protecting only the node WebSocket would leave alternate routes as bypasses.

The trusted server/runner must make policy decisions outside model-controlled text and node-controlled code. Neither a prompt nor an agent-generated manifest can approve its own access.

## Audit without turning routine work into a ceremony

Show which runner launched a node, its identity, execution mode, selected profile, effective grants, and requested access that was denied. Record approval changes, credential lifecycle, and operation outcomes without logging tokens or sensitive payloads by default.

The operator should be able to understand:

- Why this node was allowed to start without a prompt.
- Which files/network access it has directly.
- Which capabilities it can invoke through Alloy and where they execute.
- How to stop it and revoke its access.

Processes under the same OS user may be able to steal credentials or interfere with each other. Per-node identity improves attribution and revocation but does not itself prove which code used a stolen token. Stronger separation requires a real execution boundary.

## Initial implementation boundary

Keep the first implementation small:

1. Explicit trusted-process mode, clearly labeled as not isolated.
2. One paired runner and a file-backed reusable profile; no database prerequisite.
3. Server-issued, scoped node sessions with expiry and revocation behavior.
4. One tested container path, verified with the Codex node and without host credential mounts.
5. Consistent authorization on all resource/event/gateway paths and clear denials outside the profile.

File-backed configuration can survive a restart while sessions remain in memory. If sessions are memory-backed, define that a server restart invalidates them and requires authenticated renewal/reconnection; do not promise durable sessions or jobs. A full certificate enrollment/rotation system is a separate implementation decision, not a prerequisite to every local experiment.

Acceptance tests should demonstrate that an approved new node needs no repeated prompts; requests outside its profile are denied; unknown/expired credentials fail; revoked runners lose delegated access; a container cannot read a seeded host-only credential; unsupported isolation does not fall back silently; and logs contain no seeded secrets.

The existing loopback prototype and its self-declared node names do not satisfy this design yet. A successful fresh-HOME Codex run proves environment separation, not secure authentication of arbitrary programs or container isolation.
