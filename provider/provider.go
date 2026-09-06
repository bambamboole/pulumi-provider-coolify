package provider

import (
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// New builds the Coolify provider, wiring configuration and all supported
// resources into the schema.
func New() (p.Provider, error) {
	provider, err := infer.NewProviderBuilder().
		WithConfig(infer.Config(&Config{})).
		// Enum types pick up the Go package name as their module; publish them
		// alongside the resources instead.
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{"provider": "index"}).
		WithResources(
			infer.Resource(Project{}),
			infer.Resource(TeamSharedVariable{}),
			infer.Resource(ProjectSharedVariable{}),
			infer.Resource(EnvironmentSharedVariable{}),
			infer.Resource(ServerSharedVariable{}),
			infer.Resource(Database{}),
			infer.Resource(PrivateKey{}),
			infer.Resource(Server{}),
			infer.Resource(S3Storage{}),
			infer.Resource(Application{}),
			infer.Resource(Deployment{}),
			infer.Resource(ScheduledTask{}),
			infer.Resource(GitHubApp{}),
			infer.Resource(Service{}),
			infer.Resource(DatabaseBackup{}),
			infer.Resource(Storage{}),
			infer.Resource(VolumeBackup{}),
			infer.Resource(NotificationSlack{}),
			infer.Resource(NotificationDiscord{}),
			infer.Resource(NotificationEmail{}),
			infer.Resource(NotificationTelegram{}),
			infer.Resource(NotificationPushover{}),
			infer.Resource(NotificationWebhook{}),
		).
		WithFunctions(
			infer.Function(GetStorage{}),
		).
		WithDisplayName("Coolify").
		WithDescription("Manage resources on a Coolify instance: projects, shared variables, databases and their backups, private keys, GitHub Apps, servers, S3 storage, applications, services, storages and their volume backups, scheduled tasks and deployments.").
		WithPublisher("bambamboole").
		WithRepository("https://github.com/bambamboole/pulumi-provider-coolify").
		WithHomepage("https://coolify.io").
		WithLicense("Apache-2.0").
		// Let Pulumi download the plugin binary from GitHub Releases on demand.
		// release-please tags releases vX.Y.Z and the archive is named
		// pulumi-resource-coolify-vX.Y.Z-<os>-<arch>.tar.gz, matching Pulumi's
		// plugin naming. The $%7BVERSION%7D placeholder is interpolated by the
		// Pulumi CLI on download.
		WithPluginDownloadURL("https://github.com/bambamboole/pulumi-provider-coolify/releases/download/v$%7BVERSION%7D").
		Build()
	if err != nil {
		return p.Provider{}, err
	}
	provider.DiffConfig = diffTagConfig(provider.DiffConfig)
	provider.Check = checkSecretUnknowns(provider.Check)
	return provider, nil
}
