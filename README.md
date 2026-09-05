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

Requires Go 1.26+ and the Pulumi CLI.

```sh
make build       # builds bin/pulumi-resource-coolify
make test        # runs unit and reconcile tests against in-memory fakes
make schema      # writes schema.json
make gen-sdk     # regenerates the TypeScript SDK under sdk/nodejs
```

## Using the provider locally (no release needed)

You can iterate without pushing tags or publishing anything. Two pieces are involved: the **plugin binary** (resolved from Pulumi's plugin cache) and the **TypeScript SDK** (resolved from a `file:` dependency).

1. Build and install the plugin into the local cache:

   ```sh
   make install-local
   ```

   This builds the binary, packs it with `pulumi-plugin.json` and runs
   `pulumi plugin install resource coolify 0.1.0 --file ...`. Re-running it
   replaces the previously installed plugin (same version), so changes to the
   provider are picked up after the next `pulumi up`.

2. Reference the checked-in SDK from your TypeScript Pulumi project's
   `package.json`:

   ```json
   {
     "dependencies": {
       "@bambamboole/coolify": "file:/absolute/path/to/pulumi-provider-coolify/sdk/nodejs",
       "@pulumi/pulumi": "^3.0.0"
     }
   }
   ```

   npm installs a symlink and runs the SDK's `prepare` script (tsc), so the
   compiled JS is always up to date with the SDK sources.

3. Point the provider at your Coolify instance and use the resources:

   ```ts
   import * as coolify from "@bambamboole/coolify";

   const provider = new coolify.Provider("coolify", {
       baseUrl: "https://coolify.example.com",
       apiToken: process.env.COOLIFY_API_TOKEN, // secret
   });

   const project = new coolify.Project("main", {
       name: "Artisan OS",
       description: "Main project",
       environments: ["production", "development"],
   }, { provider, protect: true });
   ```

   The version the provider reports must match what Pulumi expects; both come
   from the checked-in `pulumi-plugin.json`/`package.json` `0.1.0`. Bump
   `VERSION` in the Makefile and the two files when iterating on a branch that
   changes the schema.

4. After a schema change, regenerate both artifacts and reinstall:

   ```sh
   make install-local && make gen-sdk
   ```

## Publishing a release

1. Commit and push, then tag a version:

   ```sh
   git tag v0.1.1 && git push origin v0.1.1
   ```

2. `.github/workflows/release.yml` runs on every `v*` tag:
   - builds the multi-platform plugin binaries with [goreleaser](.goreleaser.yml)
   - uploads per-OS archives, `SHA256SUMS.txt`, and `schema.json` to the GitHub release
   - publishes the TypeScript SDK to npm as `@bambamboole/coolify` **only if** the `NPM_TOKEN` repository secret is set; otherwise it dry-runs `npm pack`.

Repository secrets:
- `NPM_TOKEN` — npm token with publish rights for the `@bambboole` scope
  (optional; without it, SDK publishing is skipped).
- `GITHUB_TOKEN` — automatic, no action needed.

The schema job in `.github/workflows/check.yml` runs on every pushPR and
ensures `pulumi package get-schema` succeeds; the `sdk` job ensures the
generated SDK still compiles.

## Configuration

| Config | Type | Description |
|---|---|---|
| `baseUrl` | string | Base URL of the Coolify instance, without `/api/v1`. |
| `apiToken` | string | Read/write Coolify API token (secret). |

## How reconcile works

Each resource implements the full Pulumi lifecycle (`Create`, `Diff`, `Update`, `Read`, `Delete`) against the Coolify API:

- `Create` finds an existing resource by name and adopts it, or creates a new one.
- `Update` only `PATCH`es the fields that actually changed.
- Sensitive values (private key material, S3 credentials, database passwords) are marked `provider:"secret"` and are never persisted in state.
- `Delete` tolerates 404s so re-application after external deletion stays idempotent.
- Deployments are immutable history in Coolify: deleting a `coolify.Deployment` resource is a no-op.

## References

- Field shapes cross-checked against the Coolify v4 OpenAPI spec and the [`coollabsio/coolify-cli`](https://github.com/coollabsio/coolify-cli) Go models (MIT) as reference. See `NOTICE`.
- Provider framework: [`pulumi-go-provider`](https://github.com/pulumi/pulumi-go-provider).

## License

Apache-2.0.