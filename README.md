# pulumi-provider-coolify

A native [Pulumi](https://www.pulumi.com) provider for [Coolify](https://coolify.io) v4, written in Go on top of [`pulumi-go-provider/infer`](https://github.com/pulumi/pulumi-go-provider). It manages Coolify resources through the public API (`/api/v1`) using a client generated from Coolify's OpenAPI specification.

## Resources

| Resource | Manages | Identity for adoption |
| --- | --- | --- |
| `coolify.Project` | Project and the environments declared on it (environments are only added, never removed) | project name |
| `coolify.Database` | Standalone PostgreSQL, MySQL, MariaDB, MongoDB, Redis, KeyDB, Dragonfly or ClickHouse database | database name within the environment |
| `coolify.DatabaseBackup` | Scheduled backup configuration of a database, optionally uploaded to an S3 storage | frequency string within the database |
| `coolify.PrivateKey` | SSH private key | key name |
| `coolify.GitHubApp` | GitHub App source for private repositories | app name |
| `coolify.Server` | Server connected over SSH | server name |
| `coolify.S3Storage` | S3-compatible storage destination, e.g. an R2 bucket | storage name |
| `coolify.Application` | Application from a public or private git repository, a Docker image or a Dockerfile, including its environment variables | application name within the environment |
| `coolify.Service` | Service from a one-click template or a docker compose file, including its environment variables | service name within the environment |
| `coolify.Storage` | Persistent volume or directory mount of an application or database | mount path within the owner |
| `coolify.VolumeBackup` | Backup schedule of a persistent volume or directory mount of an application, database or service, optionally uploaded to an S3 storage | the storage (one schedule per storage) |
| `coolify.ScheduledTask` | Cron task on an application | task name within the application |
| `coolify.Deployment` | Triggers a deployment and waits for it to finish | none (one deployment per input change) |
| `coolify.NotificationEmail` | SMTP, Resend or instance email notifications | token team / `email` |
| `coolify.NotificationDiscord` | Discord webhook notifications | token team / `discord` |
| `coolify.NotificationSlack` | Slack-compatible webhook notifications, including Mattermost | token team / `slack` |
| `coolify.NotificationTelegram` | Telegram bot notifications with optional event thread IDs | token team / `telegram` |
| `coolify.NotificationPushover` | Pushover notifications | token team / `pushover` |
| `coolify.NotificationWebhook` | Generic JSON webhook notifications | token team / `webhook` |

The function `coolify.getStorage` looks up a storage of an application, database or service by mount path and/or volume name, e.g. a volume declared in a service's compose file.

On **create**, resources with an adoption identity reuse a matching Coolify resource and reconcile its settings with the declared inputs. Deployment resources trigger an action; notification resources adopt the token team's settings for their channel. Updates and deletes use the recorded API identity. Resources addressed by UUID keep that identity when renamed.

Optional inputs left unset are treated as unmanaged: Coolify's own default is kept and never overwritten, and `pulumi refresh` does not report it as drift. Inputs that must change the identity of a resource (server, database type, application source, service type) replace it. Changing `projectUuid` or `environmentName` on an application, database or service **moves it in place** instead, see below.

## Provider configuration

| Setting | Environment variable | Description |
| --- | --- | --- |
| `baseUrl` | `COOLIFY_BASE_URL` | Base URL of the Coolify instance, e.g. `https://coolify.example.com` |
| `apiToken` | `COOLIFY_API_TOKEN` | Read/write API token (Coolify > Security > API tokens) |
| `defaultTags` | | Tags attached to every application, database and service. Defaults to `["pulumi"]` |
| `disableDefaultTags` | | Attach no default tags at all (an empty `defaultTags` list counts as unset) |

```ts
import * as pulumi from "@pulumi/pulumi";
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

// Nightly dump of the database to S3, and a volume backup of a compose service.
const s3 = new coolify.S3Storage("backups", { /* ... */ }, { provider });

new coolify.DatabaseBackup("main-db", {
    databaseUuid: db.uuid,
    frequency: "0 3 * * *",
    saveS3: true,
    s3StorageUuid: s3.uuid,
    retentionAmountS3: 30,
}, { provider });

const gitea = new coolify.Service("gitea", {
    type: "gitea-with-mysql",
    projectUuid: project.uuid,
    environmentName: "production",
    serverUuid: server.uuid,
}, { provider });

new coolify.VolumeBackup("gitea-data", {
    serviceUuid: gitea.uuid,
    mountPath: "/data",
    volumeName: "gitea-data",
    frequency: "daily",
    saveS3: true,
    s3StorageUuid: s3.uuid,
    stopDuringBackup: true,
}, { provider, retainOnDelete: true });
```

## Notifications

Notification resources require **Coolify v4.3.0 or newer**. Each channel has one settings object for the provider API token's team, shared across that team's projects. Declare each team/channel combination in only one Pulumi resource. Import IDs have the form `<teamId>/<channel>`, for example `1/slack`; the provider verifies that the token belongs to that team. Coolify's Root Team has ID `0`, so its Slack notification import ID is `0/slack`.

For Mattermost, create an incoming webhook and expose its URL from a Pulumi ESC secret as the `mattermostWebhookUrl` Pulumi config key. With the provider above:

```ts
const config = new pulumi.Config();

new coolify.NotificationSlack("mattermost", {
    enabled: true,
    webhookUrl: config.requireSecret("mattermostWebhookUrl"),
    events: {
        deploymentSuccess: false,
        deploymentFailure: true,
        backupFailure: true,
        serverUnreachable: true,
    },
}, { provider });
```

**Create adopts; updates patch declared settings.** Coolify exposes GET and PATCH for these settings, with no POST or DELETE endpoints. Its GET endpoint initializes a missing settings object with defaults, so even a read can create the underlying record. The provider changes only declared inputs and event flags: unset values remain unmanaged, `false` and `0` are explicit values, and empty strings clear nullable settings by sending JSON `null`. Updates do not send test notifications.

**Destroy disables delivery and retains configuration.** It leaves credentials, URLs and event choices in Coolify. Email destroy disables all three delivery modes: `smtpEnabled`, `resendEnabled` and `useInstanceEmailSettings`. Set `retainOnDelete: true` to keep delivery enabled when removing a resource from Pulumi.

**Hidden notification fields are Pulumi secrets.** This includes webhook URLs; SMTP sender address/name, recipients, host, username and password; the Resend API key; Telegram token, chat ID and every thread ID; and Pushover user key/API token. Coolify returns these fields only when the token has `read:sensitive` or `root` and its user is a team admin/owner. When fields are omitted, the provider preserves their previous state and compares declared changes against that state so secret rotation still works. An explicit `null` response means a setting was cleared; refresh records that drift so a subsequent update can restore a declared value.

## Behaviour worth knowing

- **Moving between projects and environments is safe.** Changing `projectUuid` or `environmentName` on a `coolify.Application`, `coolify.Database` or `coolify.Service` calls Coolify's move endpoint (Coolify v4.2.0 or newer). The target environment must already exist, e.g. through the `environments` of a `coolify.Project`. The move is purely organizational: containers keep running, nothing is redeployed, and shared environment variables of the new environment apply on the next deployment. Moves made in the Coolify UI are not detected by `pulumi refresh`.
- **Database backups are adopted by frequency.** `coolify.DatabaseBackup` adopts an existing configuration whose frequency string is identical; databases created in the Coolify UI come with a `0 0 * * *` schedule that can be adopted this way. Coolify never reports the configured S3 storage, so `s3StorageUuid` is applied when it changes but drift on it is not detected. Redis, KeyDB and Dragonfly reject backup configurations.
- **Volume backups are write-only and destroy deletes the archives.** Coolify has no endpoint to read a volume backup schedule, so `coolify.VolumeBackup` re-sends the complete schedule on every update and `pulumi refresh` only detects a vanished storage, not changes made in the UI. Destroying the resource deletes the schedule **and all local and S3 archives**; set `retainOnDelete` to keep them. Deleting a `coolify.Storage` fails while a schedule exists, so make the backup depend on the storage. Single-file mounts cannot be backed up.
- **Service compose files are write-only.** Coolify hides `dockerCompose` from the API, so it is sent on create and whenever the input changes, but drift on it is not detected.
- **Private keys cannot be updated.** Coolify's `PATCH /security/keys` endpoint cannot address a key, so changing `name` or `privateKey` replaces the key and `description` is only applied on create.
- **Tags are managed by declaration.** Applications, databases and services carry the provider's `defaultTags` plus their own `tags`. Declared tags are attached (Coolify creates unknown tags on the fly and lower-cases names), tags removed from the declaration are detached, and tags added in the Coolify UI are left alone. Coolify deletes a tag once no resource carries it, so there is no standalone tag resource.
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

`internal/coolify/api` is generated with [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) from the checked-in `openapi.yaml`, a snapshot of [Coolify's specification](https://github.com/coollabsio/coolify/blob/main/openapi.yaml). Only the operations listed in `oapi-codegen.yaml` are generated; add an operation ID there when a resource needs a new endpoint. `internal/coolify` wraps the generated client with authentication, error handling, retries and hand-written models where the specification is untyped (databases, S3 storages, GitHub Apps, environments, notification settings).

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
