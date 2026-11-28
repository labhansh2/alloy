# Provider integrations

Alloy manages access and server-side authentication without translating inference APIs. A provider registration is a named upstream connection, not a model catalog or an agent harness.

## Package boundaries

- `alloy/provider`: public configuration, connection settings, and authentication helpers. Custom integrations return `provider.Config` and may supply their own `Auth`, `CheckAuth`, and `Login` callbacks.
- `alloy/provider/openai`, `alloy/provider/google`: optional first-party registration defaults and native SDK setup.
- `internal/providerauth`: shared environment-key resolution and preflight implementation for first-party integrations, not an extension contract.
- `internal/providerproxy` and `internal/providerwire`: gateway enforcement and Alloy wire details, not public SDK APIs.

Encrypted OpenAI credential storage and OAuth policy remain inside its integration. Google currently uses server-provisioned environment keys; it has no Alloy-managed persistence or browser login. A public managed credential-store interface has not been designed.

## Register Gemini

Provision `GOOGLE_API_KEY` (or `GEMINI_API_KEY`) in the **server's protected environment**. Do not embed real keys in source or pass them through command arguments. Google prefers `GOOGLE_API_KEY` when both variables are nonempty; an invalid preferred value fails rather than silently falling back.

```go
import (
    "alloy/provider/google"
    "alloy/server"
)

srv := server.New()

if err := srv.AddProvider("google", google.Provider()); err != nil {
    return err
}

if err := srv.AddClient("worker", server.Client{
    Token: protectedAlloyToken,
    Providers: []string{"google"},
}); err != nil {
    return err
}

return srv.Run(ctx)
```

`protectedAlloyToken` is a separate, randomly generated Alloy client credential of at least 32 non-whitespace ASCII characters. It is not the provider's API key.

Registration has no credential-reading or network side effects. `Run` performs a local presence/syntax check before listening; a real request may still fail because a key is revoked, lacks access, or has no quota. This integration does not install a login callback. Provision its API key explicitly rather than expecting `srv.Login` to launch a browser or save them.

## Use native SDKs on a node

Given an authorized `*node.Node` named `n`, Gemini uses `google.golang.org/genai`:

```go
connection, err := n.Provider(ctx, "google")
if err != nil {
    return err
}

client, err := google.Client(ctx, connection)
if err != nil {
    return err
}

response, err := client.Models.GenerateContent(ctx, model, genai.Text("Hello"), nil)
```

The application chooses `model` from those available to its upstream account and handles the native result/error. No Alloy model names or normalized response types are introduced. Native HTTP streaming is also supported through `Models.GenerateContentStream`; no idle node session is required.

## Authentication boundaries and limitations

- Node SDK helpers use the Alloy connection, not ambient API keys, Google Vertex/backend/endpoint defaults.
- Gemini uses the Developer API with an explicit `v1beta` version. Its SDK sends a nonsecret placeholder API key plus the Alloy bearer token to the gateway; the gateway removes both before applying the server's Google key.
- Gemini's server authentication removes caller `key`/`access_token` query credentials and `X-Goog-User-Project` billing selection while preserving other query parameters.
- HTTP retries are disabled for the demonstrated Gemini Models methods. Callers still own cancellation, streaming cleanup, and application retry policy. Other native SDK services have not been compatibility-tested.
- This is not Vertex AI, Google ADC/workload identity, or an Antigravity harness integration. Gemini Live/WebSocket sessions and resumable upload compatibility are not implemented or promised.
- Existing gateway limits still apply: provider-wide grants, bounded request bodies and stream duration, no protocol upgrades or redirects. See [server structure](../docs/server-structure.md#implemented-provider-connections).

## Custom API-key services

Custom providers do not require an Alloy-maintained package:

```go
auth, err := provider.APIKey("X-Custom-Key", serverOwnedKey)
if err != nil {
    return err
}

return srv.AddProvider("custom", provider.Config{
    BaseURL: "https://inference.example.com",
    Auth: auth,
})
```

`serverOwnedKey` must come from protected server configuration. `provider.APIKey` only applies a static header; it does not read the environment, acquire credentials, store them, or refresh them. `provider.BearerToken` handles bearer headers. For custom OAuth, signing, refresh, or setup, provide callbacks using the same public `provider.Config`; no `internal/` imports are required.

Protocol references: [Gemini API keys](https://ai.google.dev/gemini-api/docs/api-key), and [Google Gen AI Go SDK](https://github.com/googleapis/go-genai).
