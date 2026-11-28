# Alpha SDK examples

Two small programs using the public Go SDK and the root Go module:

- `server/main.go`: register OpenAI, grant a configured client access, and run.
- `node/main.go`: request that connection and stream one native OpenAI Responses call using `gpt-6-luna`.

The server exposes only `-login`; the node exposes only `-server`. Provider, model, prompt, and client name are fixed in source. Credentials are not hardcoded. There is no optional handshake-only mode.

## Configure the client token

Copy each template to an ignored local environment file:

```sh
cp examples/alpha/server/.env.example examples/alpha/server/.env
cp examples/alpha/node/.env.example examples/alpha/node/.env
```

Generate a random token (for example, `openssl rand -hex 32`) and set the same `ALLOY_CLIENT_TOKEN` in both files. Never commit the real token. This Alloy credential is separate from OpenAI authentication; the node never needs the upstream OpenAI key.

Both programs load `.env` from their **working directory**, without overriding exported variables. The commands below run inside each example directory to use its own environment file. You can instead run from the repository root with exported variables or a root-local `.env`.

## Authenticate the server

An existing managed OpenAI login is reused. For one explicit browser login:

```sh
cd examples/alpha/server
go run . -login
```

The fixed login method is `browser` for eligible ChatGPT-plan access. To explicitly save an API key instead, change that call's method to `token`; input is hidden. Login exits after setup and stores credentials outside the repository. Normal startup never opens a browser.

Alternatively, export `OPENAI_API_KEY` in the server's environment when no managed login is saved. Never put an upstream secret in source. See [OpenAI setup](../../docs/server-structure.md#openai-login-and-the-minimal-example) for eligibility, storage, and request constraints.

## Run

From the repository root, start the server without the login flag:

```sh
cd examples/alpha/server
go run .
```

In another terminal starting at the repository root, use its printed URL (or set `ALLOY_SERVER_URL` in the node environment file):

```sh
cd examples/alpha/node
go run . -server http://127.0.0.1:PORT
```

The node calls `callModel(ctx, n)`, prints the response, and exits. The hardcoded model must be available to the server's account. Edit the code to experiment; these examples intentionally do not expose a configuration interface.

Provider and native-client tests live with the SDK, not inside the alpha applications.
