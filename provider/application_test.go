package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

func applicationArgs(projectUUID string, envVars map[string]string) ApplicationArgs {
	return ApplicationArgs{
		ProjectUUID:             projectUUID,
		EnvironmentName:         "production",
		ServerUUID:              "u-server",
		Source:                  ApplicationSourceDockerImage,
		Name:                    "Mattermost",
		Description:             "team chat",
		DockerRegistryImageName: "mattermost/mattermost-team-edition",
		DockerRegistryImageTag:  "latest",
		Domains:                 "https://chat.example.com",
		PortsExposes:            "8065",
		EnvironmentVariables:    envVars,
	}
}

func TestCreateApplicationCreatesAndPatchesSettings(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production")

	args := applicationArgs(projectUUID, map[string]string{"MM_SQLSETTINGS_DRIVERNAME": "postgres"})
	args.AutoDeployEnabled = true
	app, err := createApplication(ctx, c, args)
	if err != nil {
		t.Fatalf("createApplication: %v", err)
	}
	if *app.Name != "Mattermost" || *app.Fqdn != "https://chat.example.com" || *app.EnvironmentId != fake.environmentID(projectUUID, "production") {
		t.Fatalf("unexpected application: %+v", app)
	}
	if !*app.Settings.IsAutoDeployEnabled {
		t.Fatal("auto deploy setting was not applied through the patch")
	}
	vars, _ := c.ListApplicationEnvVars(ctx, *app.Uuid)
	if len(vars) != 1 || *vars[0].Key != "MM_SQLSETTINGS_DRIVERNAME" || *vars[0].IsPreview {
		t.Fatalf("env var not created: %+v", vars)
	}
}

func TestCreateApplicationAdoptsByNameWithinEnvironment(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production", "staging")
	fake.addApplication(map[string]any{"name": "Mattermost", "environment_id": fake.environmentID(projectUUID, "staging"), "settings": map[string]any{}})
	existing := fake.addApplication(map[string]any{
		"name": "Mattermost", "description": "team chat", "fqdn": "https://old.example.com",
		"environment_id": fake.environmentID(projectUUID, "production"), "settings": map[string]any{},
	})
	fake.addEnvVar(existing, "MM_SQLSETTINGS_DRIVERNAME", "postgres", false)
	fake.addEnvVar(existing, "MM_UNDECLARED", "keep", false)

	app, err := createApplication(ctx, c, applicationArgs(projectUUID, map[string]string{
		"MM_SQLSETTINGS_DRIVERNAME": "mysql", // existing key: must not be patched
		"MM_NEW_VAR":                "value",
	}))
	if err != nil {
		t.Fatalf("createApplication: %v", err)
	}
	if *app.Uuid != existing {
		t.Fatalf("expected adoption of %s, got %s", existing, *app.Uuid)
	}
	if *app.Fqdn != "https://chat.example.com" {
		t.Fatalf("adopted application was not patched: %+v", app)
	}
	if fake.countRequests("POST", "/api/v1/applications/dockerimage") != 0 {
		t.Fatalf("application was recreated: %v", fake.requests)
	}
	vars, _ := c.ListApplicationEnvVars(ctx, existing)
	got := map[string]string{}
	for _, env := range vars {
		got[*env.Key] = *env.Value
	}
	want := map[string]string{"MM_SQLSETTINGS_DRIVERNAME": "postgres", "MM_UNDECLARED": "keep", "MM_NEW_VAR": "value"}
	if len(got) != len(want) {
		t.Fatalf("unexpected env vars: %+v", got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("env var %s = %q, want %q", key, got[key], value)
		}
	}
}

func TestApplicationPatchIsIdempotent(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production")
	args := applicationArgs(projectUUID, nil)
	app, err := createApplication(ctx, c, args)
	if err != nil {
		t.Fatalf("createApplication: %v", err)
	}
	if body, changed := applicationPatch(app, args); changed {
		t.Fatalf("freshly reconciled application must not need a patch: %+v", body)
	}
	args.Description = ""
	body, changed := applicationPatch(app, args)
	if !changed || body.Description == nil || *body.Description != "" {
		t.Fatalf("clearing the description must be sent: %+v", body)
	}
}

func TestEnvironmentVariablesNeedUpdate(t *testing.T) {
	cases := []struct {
		name       string
		olds, news map[string]string
		want       bool
	}{
		{"new key", map[string]string{"A": "1"}, map[string]string{"A": "1", "B": "2"}, true},
		{"same keys", map[string]string{"A": "1"}, map[string]string{"A": "1"}, false},
		{"value change alone is ignored", map[string]string{"A": "1"}, map[string]string{"A": "42"}, false},
		{"removed key does not trigger", map[string]string{"A": "1", "B": "2"}, map[string]string{"A": "1"}, false},
		{"nothing declared", map[string]string{"A": "1"}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := environmentVariablesNeedUpdate(tc.olds, tc.news); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplicationCheckDefaultsNameAndValidatesSource(t *testing.T) {
	ctx := context.Background()
	check := func(fields map[string]property.Value) infer.CheckResponse[ApplicationArgs] {
		t.Helper()
		resp, err := Application{}.Check(ctx, infer.CheckRequest{Name: "chat", NewInputs: property.NewMap(fields)})
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		return resp
	}
	withSource := func(source string, extra map[string]property.Value) map[string]property.Value {
		fields := map[string]property.Value{
			"source":          property.New(source),
			"projectUuid":     property.New("u-project"),
			"environmentName": property.New("production"),
			"serverUuid":      property.New("u-server"),
		}
		for k, v := range extra {
			fields[k] = v
		}
		return fields
	}

	resp := check(withSource("docker-image", map[string]property.Value{"dockerRegistryImageName": property.New("nginx")}))
	if len(resp.Failures) != 0 || resp.Inputs.Name != "chat" {
		t.Fatalf("valid docker-image inputs must pass and default the name: %+v", resp)
	}

	resp = check(withSource("public-git", nil))
	if len(resp.Failures) != 3 {
		t.Fatalf("public-git without repository, branch and build pack must fail three times: %+v", resp.Failures)
	}
	resp = check(withSource("private-deploy-key", map[string]property.Value{
		"gitRepository": property.New("git@github.com:o/r.git"), "gitBranch": property.New("main"), "buildPack": property.New("nixpacks"),
	}))
	if len(resp.Failures) != 1 || resp.Failures[0].Property != "privateKeyUuid" {
		t.Fatalf("private-deploy-key must require privateKeyUuid: %+v", resp.Failures)
	}
	if !strings.Contains(resp.Failures[0].Reason, "private-deploy-key") {
		t.Fatalf("failure should name the source: %q", resp.Failures[0].Reason)
	}
}

func TestApplicationComposeSettingsArePatchedAndReadBack(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production")
	args := ApplicationArgs{
		ProjectUUID: projectUUID, EnvironmentName: "production", ServerUUID: "u-server",
		Source: ApplicationSourcePrivateGitHubApp, Name: "artisan-os", GitHubAppUUID: "u-github-app",
		GitRepository: "artisan-os/artisan-os", GitBranch: "main", BuildPack: "dockercompose",
		DockerComposeLocation: "/compose-production.yml",
		DockerComposeDomains: map[string]string{
			"web":    "https://artisan-os.de:8080,https://www.artisan-os.de:8080",
			"reverb": "https://sockets.artisan-os.de:8080",
		},
		ConnectToDockerNetwork: true, IncludeSourceCommitInBuild: true,
	}
	app, err := createApplication(ctx, c, args)
	if err != nil {
		t.Fatalf("createApplication: %v", err)
	}
	if got := *app.DockerComposeLocation; got != "/compose-production.yml" {
		t.Fatalf("compose location = %q", got)
	}
	if !*app.Settings.ConnectToDockerNetwork || !*app.Settings.IncludeSourceCommitInBuild {
		t.Fatalf("settings were not applied: %+v", *app.Settings)
	}
	if !strings.Contains(*app.DockerComposeDomains, `"reverb":{"domain":"https:\/\/sockets.artisan-os.de:8080"}`) {
		t.Fatalf("compose domains were not sent as an array: %s", *app.DockerComposeDomains)
	}
	if body, changed := applicationPatch(app, args); changed {
		t.Fatalf("reconciled application must not need a patch: %+v", body)
	}
	inputs := applicationInputs(args, app)
	if inputs.DockerComposeDomains["web"] != args.DockerComposeDomains["web"] || inputs.DockerComposeLocation != args.DockerComposeLocation {
		t.Fatalf("compose settings not read back: %+v", inputs)
	}
	if !inputs.ConnectToDockerNetwork || !inputs.IncludeSourceCommitInBuild {
		t.Fatalf("settings not read back: %+v", inputs)
	}
	// Unmanaged compose inputs stay unmanaged on read.
	unmanaged := applicationInputs(ApplicationArgs{}, app)
	if unmanaged.DockerComposeDomains != nil || unmanaged.DockerComposeLocation != "" {
		t.Fatalf("unmanaged compose inputs were adopted: %+v", unmanaged)
	}
	args.DockerComposeDomains["web"] = "https://app.artisan-os.de:8080"
	body, changed := applicationPatch(app, args)
	if !changed || body.DockerComposeDomains == nil || len(*body.DockerComposeDomains) != 2 || *(*body.DockerComposeDomains)[1].Name != "web" {
		t.Fatalf("changed domains must be sent in service order: %+v", body.DockerComposeDomains)
	}
}

func TestParseComposeDomains(t *testing.T) {
	got := parseComposeDomains(`{"web":{"domain":"https:\/\/artisan-os.de:8080,https:\/\/www.artisan-os.de:8080"},"ssr":{"domain":""}}`)
	want := map[string]string{"web": "https://artisan-os.de:8080,https://www.artisan-os.de:8080"}
	if len(got) != 1 || got["web"] != want["web"] {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if parseComposeDomains("") != nil || parseComposeDomains("not json") != nil || parseComposeDomains(`{"ssr":{"domain":""}}`) != nil {
		t.Fatal("empty, invalid and domain-less input must yield nil")
	}
}

func TestOverwriteEnvironmentVariablesPatchesExistingKeys(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production")
	existing := fake.addApplication(map[string]any{
		"name": "Mattermost", "environment_id": fake.environmentID(projectUUID, "production"), "settings": map[string]any{},
	})
	fake.addEnvVar(existing, "MAIL_MAILER", "resend", false)
	fake.addEnvVar(existing, "MAIL_PORT", "465", false)
	fake.addEnvVar(existing, "MAIL_MAILER", "log", true)
	fake.addEnvVar(existing, "RESEND_KEY", "keep", false)

	args := applicationArgs(projectUUID, map[string]string{
		"MAIL_MAILER": "smtp", // existing key with another value: patched
		"MAIL_PORT":   "465",  // existing key with the same value: untouched
		"MAIL_HOST":   "smtp.mx.cloudflare.net",
	})
	args.OverwriteEnvironmentVariables = true
	if _, err := createApplication(ctx, c, args); err != nil {
		t.Fatalf("createApplication: %v", err)
	}
	vars, _ := c.ListApplicationEnvVars(ctx, existing)
	got := map[string]string{}
	for _, env := range vars {
		if *env.IsPreview {
			if *env.Value != "log" {
				t.Fatalf("preview variable must not be touched: %+v", env)
			}
			continue
		}
		got[*env.Key] = *env.Value
	}
	want := map[string]string{"MAIL_MAILER": "smtp", "MAIL_PORT": "465", "MAIL_HOST": "smtp.mx.cloudflare.net", "RESEND_KEY": "keep"}
	if len(got) != len(want) {
		t.Fatalf("unexpected env vars: %+v", got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("env var %s = %q, want %q", key, got[key], value)
		}
	}
	if n := fake.countRequests("PATCH", "/api/v1/applications/"+existing+"/envs"); n != 1 {
		t.Fatalf("expected exactly one env var patch, got %d", n)
	}
	if n := fake.countRequests("POST", "/api/v1/applications/"+existing+"/envs"); n != 1 {
		t.Fatalf("expected exactly one env var creation, got %d", n)
	}
}

func TestEnsureEnvironmentVariablesRejectsOverwriteWithoutUpdate(t *testing.T) {
	vars := envVars{list: func(context.Context) ([]api.EnvironmentVariable, error) { return nil, nil }}
	if err := ensureEnvironmentVariables(context.Background(), vars, map[string]string{"A": "1"}, true); err == nil {
		t.Fatal("expected an error for a resource without update support")
	}
}

func TestApplicationDiffComparesEnvironmentValuesOnlyWhenOverwriting(t *testing.T) {
	// Diff consults the provider's default tags.
	ctx := withDefaultTags(context.Background())
	olds := applicationArgs("u-project", map[string]string{"MAIL_MAILER": "resend"})
	news := applicationArgs("u-project", map[string]string{"MAIL_MAILER": "smtp"})
	state := ApplicationState{ApplicationArgs: olds, UUID: "u-app"}
	diff, err := Application{}.Diff(ctx, infer.DiffRequest[ApplicationArgs, ApplicationState]{ID: "u-app", State: state, Inputs: news})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff.HasChanges {
		t.Fatalf("a changed value must not diff without overwrite: %+v", diff.DetailedDiff)
	}
	news.OverwriteEnvironmentVariables = true
	olds.OverwriteEnvironmentVariables = true
	state.ApplicationArgs = olds
	diff, err = Application{}.Diff(ctx, infer.DiffRequest[ApplicationArgs, ApplicationState]{ID: "u-app", State: state, Inputs: news})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if _, ok := diff.DetailedDiff["environmentVariables"]; !ok || !diff.HasChanges {
		t.Fatalf("a changed value must diff with overwrite: %+v", diff.DetailedDiff)
	}
}

func TestApplicationSourcesAreResolvedOnReadAndAdopt(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := withClient(context.Background(), c)
	projectUUID := fake.addProject("Main", "production")
	keyUUID := fake.addPrivateKey("deploy")
	otherKeyUUID := fake.addPrivateKey("other")
	appUUID := fake.addGitHubApp("deploy-bot")
	otherAppUUID := fake.addGitHubApp("other-bot")

	// A deploy-key application: the key is read back and guarded on adoption.
	keyArgs := applicationArgs(projectUUID, nil)
	keyArgs.Name, keyArgs.Source, keyArgs.PrivateKeyUUID = "api", ApplicationSourcePrivateDeployKey, keyUUID
	keyArgs.GitRepository, keyArgs.GitBranch = "git@github.com:acme/api.git", "main"
	keyArgs.DockerRegistryImageName = ""
	keyApp, err := createApplication(ctx, c, keyArgs)
	if err != nil {
		t.Fatalf("createApplication: %v", err)
	}
	stale := keyArgs
	stale.PrivateKeyUUID = "stale"
	read, err := (Application{}).Read(ctx, infer.ReadRequest[ApplicationArgs, ApplicationState]{ID: *keyApp.Uuid, Inputs: stale})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read.Inputs.PrivateKeyUUID != keyUUID {
		t.Fatalf("Read must resolve the private key, got %q", read.Inputs.PrivateKeyUUID)
	}
	patches := fake.countRequests("PATCH", "/api/v1/applications/"+*keyApp.Uuid)
	if _, err := createApplication(ctx, c, keyArgs); err != nil {
		t.Fatalf("adopt with the same key: %v", err)
	}
	if fake.countRequests("PATCH", "/api/v1/applications/"+*keyApp.Uuid) != patches {
		t.Fatalf("adopt with the same key must not patch: %v", fake.requests)
	}
	keyArgs.PrivateKeyUUID = otherKeyUUID
	_, err = createApplication(ctx, c, keyArgs)
	if err == nil || !strings.Contains(err.Error(), "cannot change the key") {
		t.Fatalf("adopt with another key must fail clearly, got %v", err)
	}

	// A GitHub App application: the app is read back and re-applied on adoption.
	ghArgs := applicationArgs(projectUUID, nil)
	ghArgs.Name, ghArgs.Source, ghArgs.GitHubAppUUID = "site", ApplicationSourcePrivateGitHubApp, appUUID
	ghArgs.GitRepository, ghArgs.GitBranch = "acme/site", "main"
	ghArgs.DockerRegistryImageName = ""
	ghApp, err := createApplication(ctx, c, ghArgs)
	if err != nil {
		t.Fatalf("createApplication with GitHub App: %v", err)
	}
	path := "/api/v1/applications/" + *ghApp.Uuid
	stale = ghArgs
	stale.GitHubAppUUID = "stale"
	read, err = (Application{}).Read(ctx, infer.ReadRequest[ApplicationArgs, ApplicationState]{ID: *ghApp.Uuid, Inputs: stale})
	if err != nil {
		t.Fatalf("Read GitHub App application: %v", err)
	}
	if read.Inputs.GitHubAppUUID != appUUID || read.Inputs.PrivateKeyUUID != "" {
		t.Fatalf("Read must resolve the GitHub App only, got %+v", read.Inputs)
	}
	patches = fake.countRequests("PATCH", path)
	if _, err := createApplication(ctx, c, ghArgs); err != nil {
		t.Fatalf("adopt with the same app: %v", err)
	}
	if fake.countRequests("PATCH", path) != patches {
		t.Fatalf("adopt with the same app must not patch: %v", fake.requests)
	}
	ghArgs.GitHubAppUUID = otherAppUUID
	if _, err := createApplication(ctx, c, ghArgs); err != nil {
		t.Fatalf("adopt with another app: %v", err)
	}
	if fake.countRequests("PATCH", path) != patches+1 {
		t.Fatalf("adopt with another app must patch once: %v", fake.requests)
	}
	fake.mu.Lock()
	sourceID := fake.applications[*ghApp.Uuid]["source_id"]
	fake.mu.Unlock()
	if fmt.Sprint(sourceID) != fmt.Sprint(fake.githubApps[otherAppUUID]["id"]) {
		t.Fatalf("patch must switch the GitHub App, got %v", sourceID)
	}
}
