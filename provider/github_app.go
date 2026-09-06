package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// GitHubApp manages a GitHub App source used to deploy private repositories.
type GitHubApp struct{}

type GitHubAppArgs struct {
	// Name of the GitHub App in Coolify. An existing app with this name is adopted.
	Name string `pulumi:"name"`
	// GitHub organization the app is installed in. Leave unset for a personal account.
	Organization string `pulumi:"organization,optional"`
	// GitHub web URL. Defaults to https://github.com.
	HTMLURL string `pulumi:"htmlUrl,optional"`
	// GitHub API URL. Derived from htmlUrl when unset.
	APIURL string `pulumi:"apiUrl,optional"`
	// SSH user for git operations. Defaults to git.
	CustomUser string `pulumi:"customUser,optional"`
	// SSH port for git operations. Defaults to 22.
	CustomPort int `pulumi:"customPort,optional"`
	// Numeric GitHub App ID.
	AppID int `pulumi:"appId"`
	// Installation ID of the app in the organization or account.
	InstallationID int `pulumi:"installationId"`
	// OAuth client ID of the app.
	ClientID string `pulumi:"clientId"`
	// OAuth client secret of the app.
	ClientSecret string `pulumi:"clientSecret" provider:"secret"`
	// Webhook secret of the app.
	WebhookSecret string `pulumi:"webhookSecret,optional" provider:"secret"`
	// UUID of the Coolify private key holding the app's private key.
	PrivateKeyUUID string `pulumi:"privateKeyUuid"`
	// Make the app available to all teams (self-hosted only).
	IsSystemWide bool `pulumi:"isSystemWide,optional"`
}

type GitHubAppState struct {
	GitHubAppArgs
	// UUID of the GitHub App in Coolify.
	UUID string `pulumi:"uuid"`
	// Numeric ID Coolify uses for the app in API paths.
	InternalID int `pulumi:"internalId"`
}

func (r *GitHubApp) Annotate(a infer.Annotator) {
	a.SetToken("index", "GitHubApp")
	a.Describe(&r, "A GitHub App source for deploying private repositories. An existing app with the same name is adopted on create. Create the app on GitHub first; Coolify only stores its credentials.")
}

func (args *GitHubAppArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "Name of the GitHub App in Coolify. An existing app with this name is adopted.")
	a.Describe(&args.Organization, "GitHub organization the app is installed in. Leave unset for a personal account.")
	a.Describe(&args.HTMLURL, "GitHub web URL.")
	a.Describe(&args.APIURL, "GitHub API URL. Derived from htmlUrl when unset.")
	a.Describe(&args.CustomUser, "SSH user for git operations.")
	a.Describe(&args.CustomPort, "SSH port for git operations.")
	a.Describe(&args.AppID, "Numeric GitHub App ID.")
	a.Describe(&args.InstallationID, "Installation ID of the app in the organization or account.")
	a.Describe(&args.ClientID, "OAuth client ID of the app.")
	a.Describe(&args.ClientSecret, "OAuth client secret of the app.")
	a.Describe(&args.WebhookSecret, "Webhook secret of the app.")
	a.Describe(&args.PrivateKeyUUID, "UUID of the Coolify private key holding the app's private key (the uuid output of a PrivateKey resource).")
	a.Describe(&args.IsSystemWide, "Make the app available to all teams (self-hosted only).")
	a.SetDefault(&args.HTMLURL, "https://github.com")
	a.SetDefault(&args.CustomUser, "git")
	a.SetDefault(&args.CustomPort, 22)
}

func (state *GitHubAppState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the GitHub App in Coolify.")
	a.Describe(&state.InternalID, "Numeric ID Coolify uses for the app in API paths.")
}

func (GitHubApp) Create(ctx context.Context, req infer.CreateRequest[GitHubAppArgs]) (infer.CreateResponse[GitHubAppState], error) {
	if req.DryRun {
		return infer.CreateResponse[GitHubAppState]{Output: GitHubAppState{GitHubAppArgs: req.Inputs}}, nil
	}
	app, err := createGitHubApp(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.CreateResponse[GitHubAppState]{}, err
	}
	return infer.CreateResponse[GitHubAppState]{ID: app.UUID, Output: gitHubAppState(req.Inputs, app)}, nil
}

func (GitHubApp) Diff(ctx context.Context, req infer.DiffRequest[GitHubAppArgs, GitHubAppState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.GitHubAppArgs, req.Inputs), true), nil
}

func (GitHubApp) Update(ctx context.Context, req infer.UpdateRequest[GitHubAppArgs, GitHubAppState]) (infer.UpdateResponse[GitHubAppState], error) {
	if req.DryRun {
		return infer.UpdateResponse[GitHubAppState]{Output: GitHubAppState{GitHubAppArgs: req.Inputs, UUID: req.ID, InternalID: req.State.InternalID}}, nil
	}
	c := client(ctx)
	current, err := c.GetGitHubApp(ctx, req.ID)
	if err != nil {
		return infer.UpdateResponse[GitHubAppState]{}, err
	}
	app, err := applyGitHubApp(ctx, c, current, req.State.GitHubAppArgs, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[GitHubAppState]{}, err
	}
	return infer.UpdateResponse[GitHubAppState]{Output: gitHubAppState(req.Inputs, app)}, nil
}

func (GitHubApp) Read(ctx context.Context, req infer.ReadRequest[GitHubAppArgs, GitHubAppState]) (infer.ReadResponse[GitHubAppArgs, GitHubAppState], error) {
	app, err := client(ctx).GetGitHubApp(ctx, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[GitHubAppArgs, GitHubAppState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[GitHubAppArgs, GitHubAppState]{}, err
	}
	inputs := gitHubAppInputs(req.Inputs, app)
	return infer.ReadResponse[GitHubAppArgs, GitHubAppState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  gitHubAppState(inputs, app),
	}, nil
}

func (GitHubApp) Delete(ctx context.Context, req infer.DeleteRequest[GitHubAppState]) (infer.DeleteResponse, error) {
	if err := client(ctx).DeleteGitHubApp(ctx, req.State.InternalID); err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

// createGitHubApp adopts the app with the given name or creates it.
func createGitHubApp(ctx context.Context, c *coolify.Client, inputs GitHubAppArgs) (coolify.GitHubApp, error) {
	apps, err := c.ListGitHubApps(ctx)
	if err != nil {
		return coolify.GitHubApp{}, err
	}
	for _, app := range apps {
		if app.Name == inputs.Name {
			// Secrets and the private key are not readable, so they are always re-applied.
			return applyGitHubApp(ctx, c, app, GitHubAppArgs{}, inputs)
		}
	}
	return c.CreateGitHubApp(ctx, api.CreateGithubAppJSONRequestBody{
		Name:           inputs.Name,
		Organization:   coolify.PtrIfNonZero(inputs.Organization),
		HtmlUrl:        inputs.HTMLURL,
		ApiUrl:         coolify.PtrIfNonZero(inputs.APIURL),
		CustomUser:     coolify.PtrIfNonZero(inputs.CustomUser),
		CustomPort:     coolify.PtrIfNonZero(inputs.CustomPort),
		AppId:          inputs.AppID,
		InstallationId: inputs.InstallationID,
		ClientId:       inputs.ClientID,
		ClientSecret:   inputs.ClientSecret,
		WebhookSecret:  coolify.PtrIfNonZero(inputs.WebhookSecret),
		PrivateKeyUuid: inputs.PrivateKeyUUID,
		IsSystemWide:   coolify.PtrIfNonZero(inputs.IsSystemWide),
	})
}

// applyGitHubApp patches the fields of current that differ from the inputs.
// Secrets and the private key are compared against the previous inputs because
// the API does not return them.
func applyGitHubApp(ctx context.Context, c *coolify.Client, current coolify.GitHubApp, previous, inputs GitHubAppArgs) (coolify.GitHubApp, error) {
	var body api.UpdateGithubAppJSONRequestBody
	var patch patch
	patch.str(&body.Name, inputs.Name, current.Name)
	patch.text(&body.Organization, inputs.Organization, coolify.Deref(current.Organization))
	patch.str(&body.HtmlUrl, inputs.HTMLURL, current.HTMLURL)
	patch.str(&body.ApiUrl, inputs.APIURL, current.APIURL)
	patch.str(&body.CustomUser, inputs.CustomUser, current.CustomUser)
	patch.integer(&body.CustomPort, inputs.CustomPort, &current.CustomPort)
	patch.integer(&body.AppId, inputs.AppID, &current.AppID)
	patch.integer(&body.InstallationId, inputs.InstallationID, &current.InstallationID)
	patch.str(&body.ClientId, inputs.ClientID, current.ClientID)
	patch.str(&body.ClientSecret, inputs.ClientSecret, previous.ClientSecret)
	patch.str(&body.WebhookSecret, inputs.WebhookSecret, previous.WebhookSecret)
	patch.str(&body.PrivateKeyUuid, inputs.PrivateKeyUUID, previous.PrivateKeyUUID)
	patch.boolean(&body.IsSystemWide, inputs.IsSystemWide, current.IsSystemWide)
	if !patch.changed {
		return current, nil
	}
	if err := c.UpdateGitHubApp(ctx, current.ID, body); err != nil {
		return coolify.GitHubApp{}, err
	}
	return c.GetGitHubApp(ctx, current.UUID)
}

// gitHubAppInputs derives the inputs from the app Coolify reports, keeping the
// secrets and private key the API does not return.
func gitHubAppInputs(previous GitHubAppArgs, app coolify.GitHubApp) GitHubAppArgs {
	inputs := previous
	inputs.Name = app.Name
	inputs.Organization = coolify.Deref(app.Organization)
	inputs.HTMLURL = app.HTMLURL
	inputs.APIURL = ifSet(previous.APIURL, app.APIURL)
	inputs.CustomUser = app.CustomUser
	inputs.CustomPort = app.CustomPort
	inputs.AppID = app.AppID
	inputs.InstallationID = app.InstallationID
	inputs.ClientID = app.ClientID
	inputs.IsSystemWide = app.IsSystemWide
	return inputs
}

func gitHubAppState(inputs GitHubAppArgs, app coolify.GitHubApp) GitHubAppState {
	return GitHubAppState{GitHubAppArgs: inputs, UUID: app.UUID, InternalID: app.ID}
}
