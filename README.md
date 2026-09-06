# pulumi-provider-coolify

A native [Pulumi](https://www.pulumi.com) provider for [Coolify](https://coolify.io) v4, written in Go on top of [`pulumi-go-provider/infer`](https://github.com/pulumi/pulumi-go-provider). It manages Coolify resources through the public API (`/api/v1`) using a client generated from Coolify's OpenAPI specification.

## Resources

| Resource | Manages | Identity for adoption |
| --- | --- | --- |
| `coolify.Project` | Project and the environments declared on it (environments are only added, never removed) | project name |
| `coolify.Database` | Standalone PostgreSQL, MySQL, MariaDB, MongoDB, Redis, KeyDB, Dragonfly or ClickHouse database | database name within the environment |
| `coolify.PrivateKey` | SSH private key | key name |
| `coolify.GitHubApp` | GitHub App source for private repositories | app name |
| `coolify.Server` | Server connected over SSH | server name |
| `coolify.S3Storage` | S3-compatible storage destination, e.g. an R2 bucket | storage name |
| `coolify.Application` | Application from a public or private git repository, a Docker image or a Dockerfile, including its environment variables | application name within the environment |
| `coolify.ScheduledTask` | Cron task on an application | task name within the application |
| `coolify.Deployment` | Triggers a deployment and waits for it to finish | none (one deployment per input change) |

On **create**, every resource adopts an existing Coolify resource with the same identity instead of creating a duplicate, and reconciles its settings with the declared inputs. **Updates and deletes always address the resource by UUID**, so renaming a resource renames it in Coolify rather than creating a new one.

Optional inputs left unset are treated as unmanaged: Coolify's own default is kept and never overwritten, and `pulumi refresh` does not report it as drift. Inputs that must change the identity of a resource (project, environment, server, database type, application source) replace it.

## Provider configuration

| Setting | Environment variable | Description |
| --- | --- | --- |
| `baseUrl` | `COOLIFY_BASE_URL` | Base URL of the Coolify instance, e.g. `https://coolify.example.com` |
| `apiToken` | `COOLIFY_API_TOKEN` | Read/write API token (Coolify > Security > API tokens) |

```ts
import * as coolify from "@bambamboole/coolify";

const provider = new coolify.Provider("coolify", {
    baseUrl: "https://coolify.example.com",
    apiToken: pulumi.secret(process.env.COOLIFY_API_TOKEN!),
});

const key = new coolify.PrivateKey("deploy", {
    name: "deploy",
    privateKey: pulumi.secret(process.env.SSH_PRIVATE_KEY!),
}, { provider });

const server = new coolify.Server("app-1", {
    name: "app-1",
    ip: "203.0.113.10",
    privateKeyUuid: key.uuid,
}, { provider });

const project = new coolify.Project("main", {
    name: "Main",
    environments: ["production"],
}, { provider, protect: true });

const db = new coolify.Database("main-db", {
    type: coolify.DatabaseType.PostgreSQL,
    name: "main-db",
    projectUuid: project.uuid,
    environmentName: "production",
    serverUuid: server.uuid,
    instantDeploy: true,
}, { provider });

const app = new coolify.Application("api", {
    source: coolify.ApplicationSource.DockerImage,
    dockerRegistryImageName: "ghcr.io/acme/api",
    dockerRegistryImageTag: "1.4.2",
    projectUuid: project.uuid,
    environmentName: "production",
    serverUuid: server.uuid,
    domains: "https://api.example.com",
    portsExposes: "8080",
    environmentVariables: { DATABASE_URL: db.internalUrl },
}, { provider });

new coolify.Deployment("api", {
    application: app.uuid,
    triggers: ["1.4.2"],
}, { provider });
```

## Behaviour worth knowing

- **Private keys cannot be updated.** Coolify's `PATCH /security/keys` endpoint cannot address a key, so changing `name` or `privateKey` replaces the key and `description` is only applied on create.
- **Application environment variables are managed by key.** Declared keys that are missing in Coolify are created as hidden values; existing keys are never patched and undeclared keys are left untouched. Coolify masks hidden values, so values are never compared.
- **Deployments redeploy on any input change.** Use `triggers` to force a redeploy, e.g. with the image tag or a version. A deployment Coolify has pruned from its history keeps its recorded state.
- **Refresh drops resources that were deleted in Coolify** so the next `pulumi up` recreates them.
- **Rate limits and gateway errors are retried** with exponential backoff and `Retry-After` support.

## Development

Requires Go 1.26+, Node 22+ and the Pulumi CLI.

```sh
make build        # bin/pulumi-resource-coolify, with the version from pulumi-plugin.json
make test         # unit tests against an in-memory Coolify fake
make lint         # golangci-lint (https://golangci-lint.run/welcome/install/)
make gen-client   # regenerate internal/coolify/api from the OpenAPI snapshot
make schema       # write schema.json
make gen-sdk      # regenerate the TypeScript SDK sources in sdk/nodejs
make build-sdk    # compile the SDK into sdk/nodejs/bin (what gets published)
```

### API client

`internal/coolify/api` is generated with [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) from the checked-in `openapi.yaml`, a snapshot of [Coolify's specification](https://github.com/coollabsio/coolify/blob/main/openapi.yaml). Only the operations listed in `oapi-codegen.yaml` are generated; add an operation ID there when a resource needs a new endpoint. `internal/coolify` wraps the generated client with authentication, error handling, retries and hand-written models where the specification is untyped (databases, S3 storages, GitHub Apps, environments).

### Using the provider locally

1. Build and install the plugin into the local Pulumi plugin cache:

   ```sh
   make install-local
   ```

   Re-running it replaces the installed plugin of the same version.

2. Build the SDK and reference it from your Pulumi program:

   ```sh
   make build-sdk
   ```

   ```json
   {
     "dependencies": {
       "@bambamboole/coolify": "file:/absolute/path/to/pulumi-provider-coolify/sdk/nodejs/bin"
     }
   }
   ```

   The plugin version reported by the binary, `pulumi-plugin.json` and the SDK's `package.json` must match; release-please keeps them in sync on releases.

## Releasing

Merging a release-please PR tags `vX.Y.Z`; the release workflow builds the plugin for all platforms with goreleaser, attaches `schema.json`, and publishes `@bambamboole/coolify` to npm from `sdk/nodejs/bin`. Pulumi downloads the plugin from the GitHub release on demand.
