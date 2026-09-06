package provider

import (
	"context"
	"errors"
	"os"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

const (
	envBaseURL  = "COOLIFY_BASE_URL"
	envAPIToken = "COOLIFY_API_TOKEN"
)

// Config holds the provider-level configuration.
type Config struct {
	// BaseURL is the base URL of the Coolify instance without the API path.
	BaseURL string `pulumi:"baseUrl,optional"`
	// ApiToken is the Coolify read/write API token.
	ApiToken string `pulumi:"apiToken,optional" provider:"secret"`
	// DefaultTags are attached to every application, database and service.
	DefaultTags []string `pulumi:"defaultTags,optional"`
	// DisableDefaultTags turns default tags off entirely.
	DisableDefaultTags bool `pulumi:"disableDefaultTags,optional"`

	client *coolify.Client
}

func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c, "Manage resources on a Coolify instance through the Coolify v4 API.")
	a.Describe(&c.BaseURL, "Base URL of the Coolify instance without the API path, e.g. https://coolify.example.com. Defaults to the COOLIFY_BASE_URL environment variable.")
	a.Describe(&c.ApiToken, "Coolify read/write API token (Coolify > Security > API tokens). Defaults to the COOLIFY_API_TOKEN environment variable.")
	a.Describe(&c.DefaultTags, `Tags attached to every application, database and service in addition to their own tags. Defaults to ["pulumi"]; use disableDefaultTags to attach none.`)
	a.Describe(&c.DisableDefaultTags, "Attach no default tags at all. Needed because an empty defaultTags list is indistinguishable from an unset one.")
	a.SetDefault(&c.BaseURL, "", envBaseURL)
	a.SetDefault(&c.ApiToken, "", envAPIToken)
}

// diffTagConfig keeps the inferred property diff (including unknowns and secrets)
// but allows tag settings to change in place. infer otherwise replaces providers
// on every config change, including the false value Check adds on a 0.6 upgrade.
// Connection changes retain their replacement behavior because they may target
// another Coolify instance or team.
func diffTagConfig(diffConfig func(context.Context, p.DiffRequest) (p.DiffResponse, error)) func(context.Context, p.DiffRequest) (p.DiffResponse, error) {
	return func(ctx context.Context, req p.DiffRequest) (p.DiffResponse, error) {
		response, err := diffConfig(ctx, req)
		if err != nil {
			return response, err
		}
		for key, diff := range response.DetailedDiff {
			if key != "disableDefaultTags" && key != "defaultTags" && !strings.HasPrefix(key, "defaultTags[") {
				continue
			}
			switch diff.Kind {
			case p.AddReplace:
				diff.Kind = p.Add
			case p.DeleteReplace:
				diff.Kind = p.Delete
			case p.UpdateReplace:
				diff.Kind = p.Update
			}
			response.DetailedDiff[key] = diff
		}
		return response, nil
	}
}

// Configure validates the configuration and builds the API client once per
// provider process.
func (c *Config) Configure(_ context.Context) error {
	baseURL := firstNonEmpty(c.BaseURL, os.Getenv(envBaseURL))
	token := firstNonEmpty(c.ApiToken, os.Getenv(envAPIToken))
	if baseURL == "" {
		return errors.New("coolify: missing base URL; set the provider's baseUrl or the " + envBaseURL + " environment variable")
	}
	if token == "" {
		return errors.New("coolify: missing API token; set the provider's apiToken or the " + envAPIToken + " environment variable")
	}
	client, err := coolify.New(baseURL, token)
	if err != nil {
		return err
	}
	c.client = client
	return c.applyDefaultTags()
}

// defaultDefaultTags is used when defaultTags is not configured. Pulumi decodes
// an empty list to nil, so disableDefaultTags is the way to turn them off.
var defaultDefaultTags = []string{"pulumi"}

func (c *Config) applyDefaultTags() error {
	if c.DisableDefaultTags {
		c.DefaultTags = []string{}
		return nil
	}
	if c.DefaultTags == nil {
		c.DefaultTags = defaultDefaultTags
	}
	if failures := checkTags("defaultTags", c.DefaultTags); len(failures) > 0 {
		return errors.New("coolify: " + failures[0].Reason)
	}
	c.DefaultTags = normalizeTags(c.DefaultTags)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
