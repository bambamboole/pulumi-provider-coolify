package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

func gitHubAppArgs(privateKeyUUID string) GitHubAppArgs {
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
		PrivateKeyUUID: privateKeyUUID,
	}
}

func TestCreateGitHubAppCreatesAdoptsAndPatchesByID(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	keyUUID := fake.addPrivateKey("deploy")

	app, err := createGitHubApp(ctx, c, gitHubAppArgs(keyUUID))
	if err != nil {
		t.Fatalf("createGitHubApp: %v", err)
	}
	if app.UUID == "" || app.ID == 0 || app.Name != "deploy-bot" || app.APIURL != "https://api.github.com" {
		t.Fatalf("unexpected app: %+v", app)
	}
	if app.ClientSecret != nil {
		t.Fatalf("secrets must stay hidden in responses: %+v", app)
	}

	args := gitHubAppArgs(keyUUID)
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

	inputs := gitHubAppInputs(args, adopted, "")
	if inputs.ClientSecret != "secret" || inputs.PrivateKeyUUID != keyUUID || inputs.InstallationID != 9999 {
		t.Fatalf("inputs must keep secrets and follow Coolify: %+v", inputs)
	}
	if err := c.DeleteGitHubApp(ctx, app.ID); err != nil {
		t.Fatalf("DeleteGitHubApp: %v", err)
	}
	if _, err := c.GetGitHubApp(ctx, app.UUID); !coolify.IsNotFound(err) {
		t.Fatalf("deleted app must be not found, got %v", err)
	}
}

func TestGitHubAppAdoptResolvesKeyAndKeepsSecret(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := withClient(context.Background(), c)
	keyUUID := fake.addPrivateKey("deploy")
	otherKeyUUID := fake.addPrivateKey("other")

	app, err := createGitHubApp(ctx, c, gitHubAppArgs(keyUUID))
	if err != nil {
		t.Fatalf("createGitHubApp: %v", err)
	}
	path := "/api/v1/github-apps/" + coolify.GitHubAppID(app.ID)

	// Adopting without a client secret and with the same key is a no-op.
	args := gitHubAppArgs(keyUUID)
	args.ClientSecret = ""
	args.WebhookSecret = ""
	if _, err := createGitHubApp(ctx, c, args); err != nil {
		t.Fatalf("adopt without clientSecret: %v", err)
	}
	if n := fake.countRequests("PATCH", path); n != 0 {
		t.Fatalf("adopt with unchanged key must not patch, got %v", fake.requests)
	}
	fake.mu.Lock()
	secret := fake.githubApps[app.UUID]["client_secret"]
	fake.mu.Unlock()
	if secret != "secret" {
		t.Fatalf("adopt without clientSecret must keep the stored secret, got %v", secret)
	}

	// Read resolves the private key UUID from the app's private_key_id.
	read, err := (GitHubApp{}).Read(ctx, infer.ReadRequest[GitHubAppArgs, GitHubAppState]{ID: app.UUID, Inputs: GitHubAppArgs{PrivateKeyUUID: "stale"}})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read.Inputs.PrivateKeyUUID != keyUUID || read.State.PrivateKeyUUID != keyUUID {
		t.Fatalf("Read must resolve the private key uuid, got %+v", read.Inputs)
	}
	if read.Inputs.ClientSecret != "" {
		t.Fatalf("Read must leave an unmanaged client secret empty, got %+v", read.Inputs)
	}

	// Adopting with a different key patches only the key.
	args.PrivateKeyUUID = otherKeyUUID
	adopted, err := createGitHubApp(ctx, c, args)
	if err != nil {
		t.Fatalf("adopt with other key: %v", err)
	}
	if n := fake.countRequests("PATCH", path); n != 1 {
		t.Fatalf("adopt with a changed key must patch once, got %v", fake.requests)
	}
	fake.mu.Lock()
	keyID, secret := fake.githubApps[app.UUID]["private_key_id"], fake.githubApps[app.UUID]["client_secret"]
	fake.mu.Unlock()
	if fmt.Sprint(keyID) != fmt.Sprint(adopted.PrivateKeyID) || secret != "secret" {
		t.Fatalf("patch must switch the key and keep the secret: key=%v secret=%v", keyID, secret)
	}
	if fake.countRequests("POST", "/api/v1/github-apps") != 1 {
		t.Fatalf("app was recreated: %v", fake.requests)
	}
}

func TestCreateGitHubAppRequiresClientSecret(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()

	args := gitHubAppArgs(fake.addPrivateKey("deploy"))
	args.ClientSecret = ""
	_, err := createGitHubApp(ctx, c, args)
	if err == nil || !strings.Contains(err.Error(), "clientSecret") {
		t.Fatalf("create without clientSecret must fail with a clear error, got %v", err)
	}
	if fake.countRequests("POST", "/api/v1/github-apps") != 0 {
		t.Fatalf("no app must be created: %v", fake.requests)
	}
}
