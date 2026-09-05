# pulumi-provider-coolify

A native [Pulumi](https://www.pulumi.com) provider for [Coolify](https://coolify.io) v4, written in Go on top of [`pulumi-go-provider/infer`](https://github.com/pulumi/pulumi-go-provider). It manages Coolify resources through the public Coolify API (`/api/v1`) by declaring desired state.

## Status

Early stage. Works against a live Coolify instance for:

- `coolify.Project` — projects and their environments (environments are only added, never removed)
- `coolify.Database` — standalone Redis and PostgreSQL databases inside a project environment
- `coolify.PrivateKey` — SSH private keys (adopted keys stay read-only; key material is never stored in state)
- `coolify.Server` — servers connected over SSH through a private key
- `coolify.S3Storage` — S3-compatible storage destinations (e.g. R2 buckets)
- `coolify.Application` — applications from public/private git, Docker image, or Dockerfile sources
- `coolify.Deployment` — triggers a deployment of an application and waits for completion

Resources adopt existing Coolify resources **by name** where the Coolify API supports it (projects, databases within a project/environment, private keys, servers, S3 storage), mirroring the behavior of the TypeScript dynamic resources in the artisan-os infrastructure. On Pulumi import, `pulumi refresh` re-reads state from Coolify.

## Building

Requires Go 1.26+.

```sh
make build            # builds bin/pulumi-resource-coolify
make test             # runs unit and reconcile tests
make schema           # writes schema.json via `pulumi package get-schema`
make gen-sdk          # generates the TypeScript SDK under sdk/
```

To try it from TypeScript:

```sh
pulumi package gen-sdk ./bin/pulumi-resource-coolify --language nodejs
```

## Configuration

| Config     | Type   | Description                                              |
|------------|--------|----------------------------------------------------------|
| `baseUrl`  | string | Base URL of the Coolify instance, without `/api/v1`.     |
| `apiToken` | string | Read/write Coolify API token (`provider:secret`).        |

## Quick example

```ts
import * as coolify from "./sdk";

const provider = new coolify.Provider("coolify", {
    baseUrl: "https://coolify.example.com",
    apiToken: process.env.COOLIFY_API_TOKEN,
});

const project = new coolify.Project("main", {
    name: "Artisan OS",
    description: "Main project",
    environments: ["production", "development"],
}, { provider, protect: true });
```

## How reconcile works

Each resource implements the full Pulumi lifecycle (`Create`, `Diff`, `Update`, `Read`, `Delete`) against the Coolify API:

- `Create` finds an existing resource by name and adopts it, or creates a new one.
- `Update` only PATCHes the fields that actually changed.
- Sensitive values (private key material, S3 credentials, database passwords) are marked `provider:secret` and are never persisted in state.
- `Delete` tolerates 404s so re-application after external deletion stays idempotent.
- Deployments are immutable history in Coolify: deleting a `coolify.Deployment` resource is a no-op.

## Development notes

Run `go test ./...` to run the tests, which use an in-memory fake of the Coolify API (`httptest`) so no live instance is required. The fake in `project_reconcile_test.go` also serves as a reference for the endpoints.

Field shapes were cross-checked against the Coolify v4 OpenAPI spec and the [`coollabsio/coolify-cli`](https://github.com/coollabsio/coolify-cli) Go models (MIT) as reference. See `NOTICE`.

## License

Apache-2.0. The Coolify API model shapes used as reference come from
[`coollabsio/coolify-cli`](https://github.com/coollabsio/coolify-cli), which is MIT licensed.