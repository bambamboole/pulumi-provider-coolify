package provider

import (
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// New builds the Coolify provider, wiring configuration and all supported
// resources into the schema.
func New() (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithConfig(infer.Config(Config{})).
		WithResources(
			infer.Resource(Project{}),
			infer.Resource(Database{}),
			infer.Resource(PrivateKey{}),
			infer.Resource(Server{}),
			infer.Resource(S3Storage{}),
			infer.Resource(Application{}),
			infer.Resource(Deployment{}),
		).
		WithDisplayName("Coolify").
		WithDescription("Manage resources on a Coolify instance: projects, databases, private keys, servers, S3 storage, applications and deployments.").
		WithPublisher("bambamboole").
		WithRepository("https://github.com/bambamboole/pulumi-provider-coolify").
		WithHomepage("https://coolify.io").
		WithLicense("Apache-2.0").
		// Let Pulumi download the plugin binary from GitHub Releases on demand.
		// release-please tags releases vX.Y.Z (no component prefix), and the
		// archive is named pulumi-resource-coolify-vX.Y.Z-<os>-<arch>.tar.gz,
		// matching Pulumi's standard plugin asset naming. The $%7BVERSION%7D
		// placeholder is interpolated by the Pulumi CLI on download.
		WithPluginDownloadURL("https://github.com/bambamboole/pulumi-provider-coolify/releases/download/v$%7BVERSION%7D").
		Build()
}
