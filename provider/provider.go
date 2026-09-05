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
		Build()
}
