package provider

import (
	"context"
	"testing"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

func gitHubAppArgs() GitHubAppArgs {
	return GitHubAppArgs{
		Name:           "deploy-bot",
		Organization:   "acme",
		HTMLURL:        "https://github.com",
		CustomUser:     "git",
		CustomPort:     22,
		AppID:          1234,
		InstallationID: 5678,
		ClientID:       "Iv1.client",
		ClientSecret:   "secret",
		WebhookSecret:  "hook",
		PrivateKeyUUID: "u-key",
	}
}

func TestCreateGitHubAppCreatesAdoptsAndPatchesByID(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()

	app, err := createGitHubApp(ctx, c, gitHubAppArgs())
	if err != nil {
		t.Fatalf("createGitHubApp: %v", err)
	}
	if app.UUID == "" || app.ID == 0 || app.Name != "deploy-bot" || app.APIURL != "https://api.github.com" {
		t.Fatalf("unexpected app: %+v", app)
	}
	if app.ClientSecret != nil {
		t.Fatalf("secrets must stay hidden in responses: %+v", app)
	}

	args := gitHubAppArgs()
	args.InstallationID = 9999
	adopted, err := createGitHubApp(ctx, c, args)
	if err != nil {
		t.Fatalf("second createGitHubApp: %v", err)
	}
	if adopted.UUID != app.UUID || adopted.InstallationID != 9999 {
		t.Fatalf("app was not adopted and patched: %+v", adopted)
	}
	if fake.countRequests("POST", "/api/v1/github-apps") != 1 {
		t.Fatalf("app was recreated: %v", fake.requests)
	}
	if fake.countRequests("PATCH", "/api/v1/github-apps/"+coolify.GitHubAppID(app.ID)) != 1 {
		t.Fatalf("expected one patch by numeric id, got %v", fake.requests)
	}
	fake.mu.Lock()
	secret := fake.githubApps[app.UUID]["client_secret"]
	fake.mu.Unlock()
	if secret != "secret" {
		t.Fatalf("adopt must re-apply the secret, got %v", secret)
	}

	// A no-op update against the previous inputs must not patch.
	patches := fake.countRequests("PATCH", "/api/v1/github-apps/")
	if _, err := applyGitHubApp(ctx, c, adopted, args, args); err != nil {
		t.Fatalf("applyGitHubApp: %v", err)
	}
	if fake.countRequests("PATCH", "/api/v1/github-apps/") != patches {
		t.Fatalf("no-op apply must not patch: %v", fake.requests)
	}

	inputs := gitHubAppInputs(args, adopted)
	if inputs.ClientSecret != "secret" || inputs.PrivateKeyUUID != "u-key" || inputs.InstallationID != 9999 {
		t.Fatalf("inputs must keep secrets and follow Coolify: %+v", inputs)
	}
	if err := c.DeleteGitHubApp(ctx, app.ID); err != nil {
		t.Fatalf("DeleteGitHubApp: %v", err)
	}
	if _, err := c.GetGitHubApp(ctx, app.UUID); !coolify.IsNotFound(err) {
		t.Fatalf("deleted app must be not found, got %v", err)
	}
}
