# Credential storage

`alloy/credentials` is a small public contract for **Alloy-owned** credentials. It stores opaque bytes; it does not know whether a record holds an API key, OAuth tokens, or another integration's credential schema.

`internal/credentialstore` implements the default encrypted file/keyring backend. Integration-specific login, refresh, entitlement checks, and account identity rules stay with the integration.

## Use from a custom integration

```go
store, err := credentials.NewStore(credentials.Namespace{
    Server: "home",
    Integration: "provider.custom",
    Account: "work",
})
if err != nil {
    return err
}

return store.WithRecord(ctx, func(record credentials.Record) error {
    saved, err := record.Load()
    if err != nil && !errors.Is(err, os.ErrNotExist) {
        return err
    }

    // Your integration decodes saved, obtains or refreshes credentials using
    // ctx, and serializes the complete replacement record.
    next, err := refreshOrAcquire(ctx, saved)
    if err != nil {
        return err
    }

    return record.Save(next)
})
```

Imports are `alloy/credentials`, `errors`, and `os`. The illustrative `refreshOrAcquire` function belongs to the integration; no common token format is imposed. Custom implementations can satisfy the `Store` and `Record` interfaces without importing Alloy internals. The existing first-party provider constructors do not yet expose a custom-store option.

### Lock and record semantics

- `NewStore` validates explicit namespace labels without creating directories or accessing the OS keyring.
- `WithRecord` serializes the callback across goroutines and processes. Accounts within a server/integration share a directory lock and encryption key.
- Keep load, refresh, and save in **one callback**. Separate load/save callbacks allow concurrent refresh races.
- A record is only usable during its callback. Afterwards, the default backend returns `ErrRecordClosed` from its `Load` and `Save` methods.
- `Load` returns a fresh buffer; an absent record matches `os.ErrNotExist`. An absent record does not initialize an encryption key.
- Each `Save` immediately encrypts and atomically replaces the record. A later callback error does not roll it back; this is a locked operation, not a database transaction.
- Context cancellation bounds lock acquisition. Callback code must use the context for its own network requests and cancellation. A completed save is not undone by cancellation.
- Do not recursively acquire the same server/integration lock. Payloads including encryption overhead are limited to one MiB.

## Namespaces and protection

The default location is the OS user configuration directory beneath `alloy/credentials/v1/`. Server, integration, and account labels are hashed into path components to avoid case-insensitive or reserved-filename collisions. Authenticated encryption also binds the complete versioned namespace tuple. Labels are nonsecret metadata, limited to 1–128 ASCII letters, digits, `-`, `_`, and `.`, excluding `.` and `..`.

Server labels are explicitly supplied by trusted application code, not automatically generated identities or authorization grants. This namespace separation does not stop trusted in-process code or the host user from opening another namespace.

The backend uses:

- AES-256-GCM with a fresh random 12-byte nonce for each save.
- A random encryption key stored separately in the OS credential store, under service `alloy.credentials` with an identifier derived from the storage directory.
- Private directories and files, bounded reads, private temporary files, atomic rename, and file/directory synchronization.
- A process-local gate plus an OS-backed `credentials.lock` held across the callback.
- No plaintext fallback when the keyring is unavailable. Existing encrypted records prevent silently replacing a missing encryption key; recovery must be explicit.

Credential reads reject symlinks, nonregular files, and permissive modes. Lock files and final credential directories must also be regular/private as appropriate; saves replace the destination atomically without following a destination symlink. This is not a hostile-code sandbox, a remotely accessible vault, or protection against same-user/administrator compromise or filesystem races by such an attacker. Plaintext returned to callers remains their responsibility; secure memory erasure is not promised.

## OpenAI compatibility

OpenAI now uses the same internal storage engine through its own typed adapter, but **does not migrate existing login data into the new namespace layout**. It retains:

- `alloy/providers/openai/<account>.enc` and the existing `credentials.lock`.
- Keyring service `alloy.openai` and the existing directory-derived key identifier.
- The existing nonce-plus-AES-GCM-ciphertext file format and `alloy.openai.v1:<account>` associated data.
- Its existing credential JSON schema and `host-id` file.

The legacy OpenAI namespace remains user-profile/account-scoped rather than automatically isolated per server. Explicit per-server wiring and migration need a separate design; extracting the engine must not silently invalidate or move existing credentials.

Google and Anthropic still use environment keys. This refactor adds storage building blocks, not new login/key-entry flows. Likewise, when Alloy controls an already-authenticated CLI, that CLI should normally retain ownership of its credentials and refresh lifecycle.

Key rotation, deletion/logout, backup/import tooling, alternate headless unlock sources, and a setup UI are not implemented by this store.
