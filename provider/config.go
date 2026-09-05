package provider

import (
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Config holds the provider-level configuration. All values come from Pulumi
// stack configuration and are accessed via infer.GetConfig[Config].
type Config struct {
	// BaseURL is the base URL of the Coolify instance, including scheme and
	// optionally port, without the API path (e.g. https://coolify.example.com).
	BaseURL string `pulumi:"baseUrl"`
	// ApiToken is the Coolify read/write API token.
	ApiToken string `pulumi:"apiToken,optional" provider:"secret"`
}

func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c, "Manage resources on a Coolify instance through the Coolify v4 API.")
	a.Describe(&c.BaseURL, "Base URL of the Coolify instance, without the API path (e.g. https://coolify.example.com).")
	a.Describe(&c.ApiToken, "Coolify read/write API token (Coolify > Security > API tokens).")
}
