# Provider integrations

Alloy manages access and server-side authentication without translating inference APIs. A provider registration is a named upstream connection, not a model catalog or an agent harness.

## Package boundaries

- `alloy/provider`: public configuration, connection settings, and authentication helpers. Custom integrations return `provider.Config` and may supply their own `Auth`, `CheckAuth`, and `Login` callbacks.
- `alloy/provider/openai`, `alloy/provider/google`, `alloy/provider/anthropic`: optional first-party registration defaults and native SDK setup.
- `internal/providerauth`: shared environment-key resolution and preflight implementation for first-party integrations, not an extension contract.
- `internal/providerproxy` and `internal/providerwire`: gateway enforcement and Alloy wire details, not public SDK APIs.

Encrypted OpenAI credential storage and OAuth policy remain inside its integration. Google and Anthropic currently use server-provisioned environment keys; they have no Alloy-managed persistence or browser login. A public managed credential-store interface has not been designed.

## Register Gemini and Anthropic

Provision `GOOGLE_API_KEY` (or `GEMINI_API_KEY`) and `ANTHROPIC_API_KEY` in the **server's protected environment**. Do not embed real keys in source or pass them through command arguments. Google prefers `GOOGLE_API_KEY` when both variables are nonempty; an invalid preferred value fails rather than silently falling back.

```go
import (
    "alloy/provider/anthropic"
    "alloy/provider/google"
    "alloy/server"
)

srv := server.New()

if err := srv.AddProvider("google", google.Provider()); err != nil {
    return err
}

if err := srv.AddProvider("anthropic", anthropic.Provider()); err != nil {
    return err
}

if err := srv.AddClient("worker", server.Client{
    Token: protectedAlloyToken,
    Providers: []string{"google", "anthropic"},
}); err != nil {
    return err
}

return srv.Run(ctx)
```

`protectedAlloyToken` is a separate, randomly generated Alloy client credential of at least 32 non-whitespace ASCII characters. It is not either provider's API key.

Registration has no credential-reading or network side effects. `Run` performs a local presence/syntax check before listening; a real request may still fail because a key is revoked, lacks access, or has no quota. Neither integration installs a login callback. Provision their API keys explicitly rather than expecting `srv.Login` to launch a browser or save them.

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

Anthropic uses `github.com/anthropics/anthropic-sdk-go/v2`, imported below as `native`:

```go
connection, err := n.Provider(ctx, "anthropic")
if err != nil {
    return err
}

client := anthropic.Client(connection)
message, err := client.Messages.New(ctx, native.MessageNewParams{
    Model: native.Model(model),
    MaxTokens: 256,
    Messages: []native.MessageParam{
        native.NewUserMessage(native.NewTextBlock("Hello")),
    },
})
```

The application chooses `model` from those available to its upstream account and handles the native result/error. No Alloy model names or normalized response types are introduced. Native HTTP streaming is also supported through `Models.GenerateContentStream` and `Messages.NewStreaming`; no idle node session is required.

## Authentication boundaries and limitations

- Node SDK helpers use the Alloy connection, not ambient API keys, Anthropic profiles/custom headers, or Google Vertex/backend/endpoint defaults.
- Gemini uses the Developer API with an explicit `v1beta` version. Its SDK sends a nonsecret placeholder API key plus the Alloy bearer token to the gateway; the gateway removes both before applying the server's Google key.
- Gemini's server authentication removes caller `key`/`access_token` query credentials and `X-Goog-User-Project` billing selection while preserving other query parameters.
- Anthropic's SDK sends the Alloy bearer token and its normal API-version header; the gateway replaces authentication with the server's `x-api-key`.
- HTTP retries are disabled for the demonstrated Gemini Models and Anthropic Messages methods. Callers still own cancellation, streaming cleanup, and application retry policy. Other native SDK services have not been compatibility-tested.
- This is not Vertex AI, Google ADC/workload identity, Claude subscription OAuth, or a Claude Code/Antigravity harness integration. Gemini Live/WebSocket sessions and resumable upload compatibility are not implemented or promised.
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

Protocol references: [Gemini API keys](https://ai.google.dev/gemini-api/docs/api-key), [Google Gen AI Go SDK](https://github.com/googleapis/go-genai), [Anthropic API overview](https://platform.claude.com/docs/en/api/overview), and [Anthropic Go SDK](https://github.com/anthropics/anthropic-sdk-go).
