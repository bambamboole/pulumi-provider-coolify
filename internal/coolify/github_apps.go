package coolify

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// GitHubApp is a GitHub App source. The OpenAPI specification leaves the
// response untyped, so the model is hand-written. Coolify hides client_secret
// and webhook_secret unless the token may read sensitive data.
type GitHubApp struct {
	ID             int     `json:"id"`
	UUID           string  `json:"uuid"`
	Name           string  `json:"name"`
	Organization   *string `json:"organization"`
	APIURL         string  `json:"api_url"`
	HTMLURL        string  `json:"html_url"`
	CustomUser     string  `json:"custom_user"`
	CustomPort     int     `json:"custom_port"`
	AppID          int     `json:"app_id"`
	InstallationID int     `json:"installation_id"`
	ClientID       string  `json:"client_id"`
	ClientSecret   *string `json:"client_secret"`
	WebhookSecret  *string `json:"webhook_secret"`
	PrivateKeyID   int     `json:"private_key_id"`
	IsSystemWide   bool    `json:"is_system_wide"`
	IsPublic       bool    `json:"is_public"`
}

func (c *Client) ListGitHubApps(ctx context.Context) ([]GitHubApp, error) {
	return decode[[]GitHubApp](c.api.ListGithubApps(ctx))
}

// GetGitHubApp finds a GitHub App by UUID. The API has no single-app endpoint,
// so it lists the apps and returns a 404 APIError when missing.
func (c *Client) GetGitHubApp(ctx context.Context, uuid string) (GitHubApp, error) {
	apps, err := c.ListGitHubApps(ctx)
	if err != nil {
		return GitHubApp{}, err
	}
	for _, app := range apps {
		if app.UUID == uuid {
			return app, nil
		}
	}
	return GitHubApp{}, &APIError{
		Status: http.StatusNotFound,
		Method: http.MethodGet,
		Path:   apiPath + "/github-apps/" + uuid,
		Body:   `{"message":"GitHub app not found"}`,
	}
}

func (c *Client) CreateGitHubApp(ctx context.Context, body api.CreateGithubAppJSONRequestBody) (GitHubApp, error) {
	return decode[GitHubApp](c.api.CreateGithubApp(ctx, body))
}

// UpdateGitHubApp patches the app. Coolify addresses apps by numeric ID here.
func (c *Client) UpdateGitHubApp(ctx context.Context, id int, body api.UpdateGithubAppJSONRequestBody) error {
	return check(c.api.UpdateGithubApp(ctx, id, body))
}

func (c *Client) DeleteGitHubApp(ctx context.Context, id int) error {
	return check(c.api.DeleteGithubApp(ctx, id))
}

// GitHubAppID formats the numeric ID the way the API path expects it.
func GitHubAppID(id int) string { return strconv.Itoa(id) }
