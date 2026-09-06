package provider

import (
	"context"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// ApplicationSource selects where the application is built from.
type ApplicationSource string

const (
	ApplicationSourcePublicGit        ApplicationSource = "public-git"
	ApplicationSourcePrivateDeployKey ApplicationSource = "private-deploy-key"
	ApplicationSourcePrivateGitHubApp ApplicationSource = "private-github-app"
	ApplicationSourceDockerImage      ApplicationSource = "docker-image"
	ApplicationSourceDockerfile       ApplicationSource = "dockerfile"
)

func (ApplicationSource) Values() []infer.EnumValue[ApplicationSource] {
	return []infer.EnumValue[ApplicationSource]{
		{Name: "PublicGit", Value: ApplicationSourcePublicGit, Description: "Public git repository."},
		{Name: "PrivateDeployKey", Value: ApplicationSourcePrivateDeployKey, Description: "Private git repository accessed with a Coolify private key."},
		{Name: "PrivateGitHubApp", Value: ApplicationSourcePrivateGitHubApp, Description: "Private GitHub repository accessed through a GitHub App."},
		{Name: "DockerImage", Value: ApplicationSourceDockerImage, Description: "Prebuilt image from a Docker registry."},
		{Name: "Dockerfile", Value: ApplicationSourceDockerfile, Description: "Inline Dockerfile."},
	}
}

// Application manages a Coolify application inside a project environment.
type Application struct{}

type ApplicationArgs struct {
	// UUID of the Coolify project the application belongs to.
	ProjectUUID string `pulumi:"projectUuid"`
	// Name of the environment inside the project.
	EnvironmentName string `pulumi:"environmentName"`
	// UUID of the server hosting the application.
	ServerUUID string `pulumi:"serverUuid"`
	// Where the application is built from.
	Source ApplicationSource `pulumi:"source"`

	// Application name. Defaults to the Pulumi resource name.
	Name string `pulumi:"name,optional"`
	// Description of the application.
	Description string `pulumi:"description,optional"`

	// Git repository URL for git sources.
	GitRepository string `pulumi:"gitRepository,optional"`
	// Git branch to deploy for git sources.
	GitBranch string `pulumi:"gitBranch,optional"`
	// Git commit SHA to deploy. Defaults to the branch head.
	GitCommitSHA string `pulumi:"gitCommitSha,optional"`
	// Build pack for git sources: nixpacks, railpack, static, dockerfile or dockercompose.
	BuildPack string `pulumi:"buildPack,optional"`
	// UUID of the Coolify private key for private-deploy-key sources.
	PrivateKeyUUID string `pulumi:"privateKeyUuid,optional"`
	// UUID of the Coolify GitHub App for private-github-app sources.
	GitHubAppUUID string `pulumi:"githubAppUuid,optional"`
	// Image name for docker-image sources.
	DockerRegistryImageName string `pulumi:"dockerRegistryImageName,optional"`
	// Image tag for docker-image sources. Defaults to latest.
	DockerRegistryImageTag string `pulumi:"dockerRegistryImageTag,optional"`
	// Dockerfile content for dockerfile sources.
	Dockerfile string `pulumi:"dockerfile,optional"`
	// Location of the Dockerfile inside the repository.
	DockerfileLocation string `pulumi:"dockerfileLocation,optional"`

	// Comma separated domains the application is served on.
	Domains string `pulumi:"domains,optional"`
	// Port the container exposes.
	PortsExposes string `pulumi:"portsExposes,optional"`
	// Host to container port mappings.
	PortsMappings string `pulumi:"portsMappings,optional"`
	// Install command override.
	InstallCommand string `pulumi:"installCommand,optional"`
	// Build command override.
	BuildCommand string `pulumi:"buildCommand,optional"`
	// Start command override.
	StartCommand string `pulumi:"startCommand,optional"`
	// Directory inside the repository to build from.
	BaseDirectory string `pulumi:"baseDirectory,optional"`
	// Directory with the build output for static sites.
	PublishDirectory string `pulumi:"publishDirectory,optional"`

	// Deploy right after creating the application.
	InstantDeploy bool `pulumi:"instantDeploy,optional"`
	// Deploy automatically on git push.
	AutoDeployEnabled bool `pulumi:"autoDeployEnabled,optional"`
	// Redirect HTTP to HTTPS.
	ForceHTTPSEnabled bool `pulumi:"forceHttpsEnabled,optional"`
	// Deploy previews for pull requests.
	PreviewDeploymentsEnabled bool `pulumi:"previewDeploymentsEnabled,optional"`

	// Enable the container health check.
	HealthCheckEnabled bool `pulumi:"healthCheckEnabled,optional"`
	// Health check path.
	HealthCheckPath string `pulumi:"healthCheckPath,optional"`
	// Health check port.
	HealthCheckPort string `pulumi:"healthCheckPort,optional"`
	// Health check HTTP method.
	HealthCheckMethod string `pulumi:"healthCheckMethod,optional"`

	// Memory limit, e.g. "512m".
	LimitsMemory string `pulumi:"limitsMemory,optional"`
	// CPU limit, e.g. "0.5".
	LimitsCPUs string `pulumi:"limitsCPUs,optional"`
	// Tags attached to the application in addition to the provider's default tags.
	Tags []string `pulumi:"tags,optional"`

	// Environment variables managed by key: declared keys missing in Coolify are
	// created as hidden values, existing keys are never patched and undeclared
	// keys are left untouched.
	EnvironmentVariables map[string]string `pulumi:"environmentVariables,optional"`
}

type ApplicationState struct {
	ApplicationArgs
	// Tags the provider attached: the provider's default tags plus the declared ones.
	// Optional for states written before tag management was introduced.
	AppliedTags []string `pulumi:"appliedTags,optional"`
	// UUID of the application in Coolify.
	UUID string `pulumi:"uuid"`
	// FQDN Coolify serves the application on.
	FQDN string `pulumi:"fqdn"`
	// Status reported by Coolify.
	Status string `pulumi:"status"`
}

func (r *Application) Annotate(a infer.Annotator) {
	a.SetToken("index", "Application")
	a.Describe(&r, "A Coolify application built from a git repository, a Docker image, or a Dockerfile. An existing application with the same name in the environment is adopted on create.")
}

func (args *ApplicationArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ProjectUUID, "UUID of the Coolify project (the uuid output of a Project resource). Changing it moves the resource to the new project in place.")
	a.Describe(&args.EnvironmentName, "Name of the environment inside the project. Changing it moves the resource in place; the environment must already exist.")
	a.Describe(&args.ServerUUID, "UUID of the server hosting the application (the uuid output of a Server resource).")
	a.Describe(&args.Source, "Where the application is built from.")
	a.Describe(&args.Name, "Application name. Defaults to the Pulumi resource name. An existing application with this name in the environment is adopted.")
	a.Describe(&args.Description, "Description of the application.")
	a.Describe(&args.GitRepository, "Git repository URL for git sources.")
	a.Describe(&args.GitBranch, "Git branch to deploy for git sources.")
	a.Describe(&args.GitCommitSHA, "Git commit SHA to deploy. Defaults to the branch head.")
	a.Describe(&args.BuildPack, "Build pack for git sources: nixpacks, railpack, static, dockerfile or dockercompose.")
	a.Describe(&args.PrivateKeyUUID, "UUID of the Coolify private key for private-deploy-key sources. Changing it replaces the application.")
	a.Describe(&args.GitHubAppUUID, "UUID of the Coolify GitHub App for private-github-app sources. Changing it replaces the application.")
	a.Describe(&args.DockerRegistryImageName, "Image name for docker-image sources.")
	a.Describe(&args.DockerRegistryImageTag, "Image tag for docker-image sources. Defaults to latest.")
	a.Describe(&args.Dockerfile, "Dockerfile content for dockerfile sources.")
	a.Describe(&args.DockerfileLocation, "Location of the Dockerfile inside the repository.")
	a.Describe(&args.Domains, "Comma separated domains the application is served on.")
	a.Describe(&args.PortsExposes, "Port the container exposes.")
	a.Describe(&args.PortsMappings, "Host to container port mappings.")
	a.Describe(&args.InstallCommand, "Install command override.")
	a.Describe(&args.BuildCommand, "Build command override.")
	a.Describe(&args.StartCommand, "Start command override.")
	a.Describe(&args.BaseDirectory, "Directory inside the repository to build from.")
	a.Describe(&args.PublishDirectory, "Directory with the build output for static sites.")
	a.Describe(&args.InstantDeploy, "Deploy right after creating the application. Only relevant on create.")
	a.Describe(&args.AutoDeployEnabled, "Deploy automatically on git push.")
	a.Describe(&args.ForceHTTPSEnabled, "Redirect HTTP to HTTPS.")
	a.Describe(&args.PreviewDeploymentsEnabled, "Deploy previews for pull requests.")
	a.Describe(&args.HealthCheckEnabled, "Enable the container health check.")
	a.Describe(&args.HealthCheckPath, "Health check path.")
	a.Describe(&args.HealthCheckPort, "Health check port.")
	a.Describe(&args.HealthCheckMethod, "Health check HTTP method.")
	a.Describe(&args.LimitsMemory, `Memory limit, e.g. "512m".`)
	a.Describe(&args.LimitsCPUs, `CPU limit, e.g. "0.5".`)
	a.Describe(&args.Tags, "Tags attached to the application in addition to the provider's default tags. Declared tags are attached, tags removed from the declaration are detached, tags added in the Coolify UI are left untouched.")
	a.Describe(&args.EnvironmentVariables, "Environment variables managed by key. Declared keys missing in Coolify are created as hidden values; existing keys are never patched and undeclared keys are left untouched.")
}

func (state *ApplicationState) Annotate(a infer.Annotator) {
	a.Describe(&state.AppliedTags, "Tags the provider attached: the provider's default tags plus the declared ones.")
	a.Describe(&state.UUID, "UUID of the application in Coolify.")
	a.Describe(&state.FQDN, "FQDN Coolify serves the application on.")
	a.Describe(&state.Status, "Status reported by Coolify.")
}

func (Application) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ApplicationArgs], error) {
	args, failures, err := infer.DefaultCheck[ApplicationArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[ApplicationArgs]{}, err
	}
	if args.Name == "" {
		args.Name = req.Name
	}
	failures = append(failures, checkTags("tags", args.Tags)...)
	args.Tags = normalizeTags(args.Tags)
	require := func(property, value string) {
		if value == "" {
			failures = append(failures, p.CheckFailure{
				Property: property,
				Reason:   fmt.Sprintf("%s is required for source %q", property, args.Source),
			})
		}
	}
	switch args.Source {
	case ApplicationSourcePublicGit:
		require("gitRepository", args.GitRepository)
		require("gitBranch", args.GitBranch)
		require("buildPack", args.BuildPack)
	case ApplicationSourcePrivateDeployKey:
		require("gitRepository", args.GitRepository)
		require("gitBranch", args.GitBranch)
		require("buildPack", args.BuildPack)
		require("privateKeyUuid", args.PrivateKeyUUID)
	case ApplicationSourcePrivateGitHubApp:
		require("gitRepository", args.GitRepository)
		require("gitBranch", args.GitBranch)
		require("buildPack", args.BuildPack)
		require("githubAppUuid", args.GitHubAppUUID)
	case ApplicationSourceDockerImage:
		require("dockerRegistryImageName", args.DockerRegistryImageName)
	case ApplicationSourceDockerfile:
		require("dockerfile", args.Dockerfile)
	}
	return infer.CheckResponse[ApplicationArgs]{Inputs: args, Failures: failures}, nil
}

func (Application) Create(ctx context.Context, req infer.CreateRequest[ApplicationArgs]) (infer.CreateResponse[ApplicationState], error) {
	if req.DryRun {
		return infer.CreateResponse[ApplicationState]{Output: ApplicationState{ApplicationArgs: req.Inputs}}, nil
	}
	c := client(ctx)
	app, err := createApplication(ctx, c, req.Inputs)
	if err != nil {
		return infer.CreateResponse[ApplicationState]{}, err
	}
	state := applicationState(req.Inputs, app)
	if state.AppliedTags, err = reconcileTags(ctx, c, applicationOwner(state.UUID), effectiveTags(ctx, req.Inputs.Tags), nil); err != nil {
		return infer.CreateResponse[ApplicationState]{}, err
	}
	return infer.CreateResponse[ApplicationState]{ID: state.UUID, Output: state}, nil
}

func (Application) Diff(ctx context.Context, req infer.DiffRequest[ApplicationArgs, ApplicationState]) (infer.DiffResponse, error) {
	// Project and environment changes move the application in place.
	diff := diffArgs(req.State.ApplicationArgs, req.Inputs, "serverUuid", "source", "privateKeyUuid", "githubAppUuid")
	// Only relevant on create.
	delete(diff, "instantDeploy")
	// Tags are compared against what the provider applied, so a changed
	// provider default is picked up too.
	delete(diff, "tags")
	if tagsDiffer(effectiveTags(ctx, req.Inputs.Tags), req.State.AppliedTags) {
		diff["tags"] = p.PropertyDiff{Kind: p.Update}
	}
	// Environment variables are additive by key: only newly declared keys
	// trigger an update, values are never compared.
	delete(diff, "environmentVariables")
	if environmentVariablesNeedUpdate(req.State.EnvironmentVariables, req.Inputs.EnvironmentVariables) {
		diff["environmentVariables"] = p.PropertyDiff{Kind: p.Update}
	}
	return diffResponse(diff, req.State.Name == req.Inputs.Name), nil
}

func (Application) Update(ctx context.Context, req infer.UpdateRequest[ApplicationArgs, ApplicationState]) (infer.UpdateResponse[ApplicationState], error) {
	if req.DryRun {
		state := req.State
		state.ApplicationArgs = req.Inputs
		return infer.UpdateResponse[ApplicationState]{Output: state}, nil
	}
	c := client(ctx)
	current, err := c.GetApplication(ctx, req.ID)
	if err != nil {
		return infer.UpdateResponse[ApplicationState]{}, err
	}
	moved, err := ensurePlacement(ctx, c, applicationPlacement(req.State.ApplicationArgs), applicationPlacement(req.Inputs),
		coolify.Deref(current.EnvironmentId), func(ctx context.Context, environmentUUID string) error {
			return c.MoveApplication(ctx, req.ID, environmentUUID)
		})
	if err != nil {
		return infer.UpdateResponse[ApplicationState]{}, err
	}
	if moved {
		if current, err = c.GetApplication(ctx, req.ID); err != nil {
			return infer.UpdateResponse[ApplicationState]{}, err
		}
	}
	app, err := applyApplication(ctx, c, current, req.Inputs, false)
	if err != nil {
		return infer.UpdateResponse[ApplicationState]{}, err
	}
	state := applicationState(req.Inputs, app)
	if state.AppliedTags, err = reconcileTags(ctx, c, applicationOwner(req.ID), effectiveTags(ctx, req.Inputs.Tags), req.State.AppliedTags); err != nil {
		return infer.UpdateResponse[ApplicationState]{}, err
	}
	return infer.UpdateResponse[ApplicationState]{Output: state}, nil
}

func (Application) Read(ctx context.Context, req infer.ReadRequest[ApplicationArgs, ApplicationState]) (infer.ReadResponse[ApplicationArgs, ApplicationState], error) {
	c := client(ctx)
	app, err := c.GetApplication(ctx, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[ApplicationArgs, ApplicationState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[ApplicationArgs, ApplicationState]{}, err
	}
	inputs := applicationInputs(req.Inputs, app)
	if len(req.Inputs.EnvironmentVariables) > 0 {
		existing, err := c.ListApplicationEnvVars(ctx, req.ID)
		if err != nil {
			return infer.ReadResponse[ApplicationArgs, ApplicationState]{}, err
		}
		inputs.EnvironmentVariables = declaredEnvironmentVariables(req.Inputs.EnvironmentVariables, existing)
	}
	tags, applied, err := readTags(ctx, c, applicationOwner(req.ID), req.Inputs.Tags, req.State.AppliedTags)
	if err != nil {
		return infer.ReadResponse[ApplicationArgs, ApplicationState]{}, err
	}
	inputs.Tags = tags
	state := applicationState(inputs, app)
	state.AppliedTags = applied
	return infer.ReadResponse[ApplicationArgs, ApplicationState]{ID: req.ID, Inputs: inputs, State: state}, nil
}

func (Application) Delete(ctx context.Context, req infer.DeleteRequest[ApplicationState]) (infer.DeleteResponse, error) {
	if err := client(ctx).DeleteApplication(ctx, req.ID); err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

// createApplication adopts the application with the same name in the
// environment or creates it, then reconciles its settings with the inputs.
func createApplication(ctx context.Context, c *coolify.Client, inputs ApplicationArgs) (api.Application, error) {
	environment, err := resolveEnvironment(ctx, c, inputs.ProjectUUID, inputs.EnvironmentName)
	if err != nil {
		return api.Application{}, err
	}
	apps, err := c.ListApplications(ctx)
	if err != nil {
		return api.Application{}, err
	}
	for _, candidate := range apps {
		if coolify.Deref(candidate.Name) == inputs.Name && coolify.Deref(candidate.EnvironmentId) == environment.ID {
			return applyApplication(ctx, c, candidate, inputs, false)
		}
	}

	uuid, err := createApplicationBySource(ctx, c, environment, inputs)
	if err != nil {
		return api.Application{}, err
	}
	created, err := c.GetApplication(ctx, uuid)
	if err != nil {
		return api.Application{}, err
	}
	// The create endpoints only receive the identity and source; everything
	// else is applied through the same patch as an update.
	return applyApplication(ctx, c, created, inputs, inputs.InstantDeploy)
}

func createApplicationBySource(ctx context.Context, c *coolify.Client, environment coolify.Environment, inputs ApplicationArgs) (string, error) {
	name := &inputs.Name
	description := coolify.PtrIfNonZero(inputs.Description)
	instantDeploy := coolify.Ptr(false)
	switch inputs.Source {
	case ApplicationSourcePublicGit:
		return c.CreatePublicApplication(ctx, api.CreatePublicApplicationJSONRequestBody{
			ProjectUuid:     inputs.ProjectUUID,
			EnvironmentName: environment.Name,
			EnvironmentUuid: environment.UUID,
			ServerUuid:      inputs.ServerUUID,
			Name:            name,
			Description:     description,
			GitRepository:   inputs.GitRepository,
			GitBranch:       inputs.GitBranch,
			GitCommitSha:    coolify.PtrIfNonZero(inputs.GitCommitSHA),
			BuildPack:       api.CreatePublicApplicationJSONBodyBuildPack(inputs.BuildPack),
			PortsExposes:    coolify.PtrIfNonZero(inputs.PortsExposes),
			InstantDeploy:   instantDeploy,
		})
	case ApplicationSourcePrivateDeployKey:
		return c.CreatePrivateDeployKeyApplication(ctx, api.CreatePrivateDeployKeyApplicationJSONRequestBody{
			ProjectUuid:     inputs.ProjectUUID,
			EnvironmentName: environment.Name,
			EnvironmentUuid: environment.UUID,
			ServerUuid:      inputs.ServerUUID,
			Name:            name,
			Description:     description,
			GitRepository:   inputs.GitRepository,
			GitBranch:       inputs.GitBranch,
			GitCommitSha:    coolify.PtrIfNonZero(inputs.GitCommitSHA),
			BuildPack:       api.CreatePrivateDeployKeyApplicationJSONBodyBuildPack(inputs.BuildPack),
			PrivateKeyUuid:  inputs.PrivateKeyUUID,
			PortsExposes:    coolify.PtrIfNonZero(inputs.PortsExposes),
			InstantDeploy:   instantDeploy,
		})
	case ApplicationSourcePrivateGitHubApp:
		return c.CreatePrivateGithubAppApplication(ctx, api.CreatePrivateGithubAppApplicationJSONRequestBody{
			ProjectUuid:     inputs.ProjectUUID,
			EnvironmentName: environment.Name,
			EnvironmentUuid: environment.UUID,
			ServerUuid:      inputs.ServerUUID,
			Name:            name,
			Description:     description,
			GitRepository:   inputs.GitRepository,
			GitBranch:       inputs.GitBranch,
			GitCommitSha:    coolify.PtrIfNonZero(inputs.GitCommitSHA),
			BuildPack:       api.CreatePrivateGithubAppApplicationJSONBodyBuildPack(inputs.BuildPack),
			GithubAppUuid:   inputs.GitHubAppUUID,
			PortsExposes:    coolify.PtrIfNonZero(inputs.PortsExposes),
			InstantDeploy:   instantDeploy,
		})
	case ApplicationSourceDockerImage:
		return c.CreateDockerImageApplication(ctx, api.CreateDockerimageApplicationJSONRequestBody{
			ProjectUuid:             inputs.ProjectUUID,
			EnvironmentName:         environment.Name,
			EnvironmentUuid:         environment.UUID,
			ServerUuid:              inputs.ServerUUID,
			Name:                    name,
			Description:             description,
			DockerRegistryImageName: inputs.DockerRegistryImageName,
			DockerRegistryImageTag:  coolify.PtrIfNonZero(inputs.DockerRegistryImageTag),
			PortsExposes:            coolify.PtrIfNonZero(inputs.PortsExposes),
			InstantDeploy:           instantDeploy,
		})
	case ApplicationSourceDockerfile:
		return c.CreateDockerfileApplication(ctx, api.CreateDockerfileApplicationJSONRequestBody{
			ProjectUuid:     inputs.ProjectUUID,
			EnvironmentName: environment.Name,
			EnvironmentUuid: environment.UUID,
			ServerUuid:      inputs.ServerUUID,
			Name:            name,
			Description:     description,
			Dockerfile:      inputs.Dockerfile,
			PortsExposes:    coolify.PtrIfNonZero(inputs.PortsExposes),
			InstantDeploy:   instantDeploy,
		})
	}
	return "", fmt.Errorf("unsupported application source %q", inputs.Source)
}

// applyApplication patches the fields of current that differ from the inputs,
// reconciles the environment variables and returns the refreshed application.
// With deploy set, the patch also triggers a deployment.
func applyApplication(ctx context.Context, c *coolify.Client, current api.Application, inputs ApplicationArgs, deploy bool) (api.Application, error) {
	uuid := coolify.Deref(current.Uuid)
	body, changed := applicationPatch(current, inputs)
	if deploy {
		body.InstantDeploy = coolify.Ptr(true)
		changed = true
	}
	if changed {
		if err := c.UpdateApplication(ctx, uuid, body); err != nil {
			return api.Application{}, err
		}
	}
	if err := ensureEnvironmentVariables(ctx, applicationEnvVars(c, uuid), inputs.EnvironmentVariables); err != nil {
		return api.Application{}, err
	}
	if !changed {
		return current, nil
	}
	return c.GetApplication(ctx, uuid)
}

func applicationPatch(current api.Application, inputs ApplicationArgs) (api.UpdateApplicationByUuidJSONRequestBody, bool) {
	var body api.UpdateApplicationByUuidJSONRequestBody
	var patch patch
	settings := coolify.Deref(current.Settings)

	patch.str(&body.Name, inputs.Name, coolify.Deref(current.Name))
	patch.text(&body.Description, inputs.Description, coolify.Deref(current.Description))
	patch.str(&body.GitRepository, inputs.GitRepository, coolify.Deref(current.GitRepository))
	patch.str(&body.GitBranch, inputs.GitBranch, coolify.Deref(current.GitBranch))
	patch.str(&body.GitCommitSha, inputs.GitCommitSHA, coolify.Deref(current.GitCommitSha))
	if inputs.BuildPack != "" && inputs.BuildPack != string(coolify.Deref(current.BuildPack)) {
		body.BuildPack = coolify.Ptr(api.UpdateApplicationByUuidJSONBodyBuildPack(inputs.BuildPack))
		patch.changed = true
	}
	patch.str(&body.DockerRegistryImageName, inputs.DockerRegistryImageName, coolify.Deref(current.DockerRegistryImageName))
	patch.str(&body.DockerRegistryImageTag, inputs.DockerRegistryImageTag, coolify.Deref(current.DockerRegistryImageTag))
	patch.str(&body.Dockerfile, inputs.Dockerfile, coolify.Deref(current.Dockerfile))
	patch.str(&body.DockerfileLocation, inputs.DockerfileLocation, coolify.Deref(current.DockerfileLocation))
	patch.str(&body.Domains, inputs.Domains, coolify.Deref(current.Fqdn))
	patch.str(&body.PortsExposes, inputs.PortsExposes, coolify.Deref(current.PortsExposes))
	patch.str(&body.PortsMappings, inputs.PortsMappings, coolify.Deref(current.PortsMappings))
	patch.str(&body.InstallCommand, inputs.InstallCommand, coolify.Deref(current.InstallCommand))
	patch.str(&body.BuildCommand, inputs.BuildCommand, coolify.Deref(current.BuildCommand))
	patch.str(&body.StartCommand, inputs.StartCommand, coolify.Deref(current.StartCommand))
	patch.str(&body.BaseDirectory, inputs.BaseDirectory, coolify.Deref(current.BaseDirectory))
	patch.str(&body.PublishDirectory, inputs.PublishDirectory, coolify.Deref(current.PublishDirectory))
	patch.boolean(&body.IsAutoDeployEnabled, inputs.AutoDeployEnabled, coolify.Deref(settings.IsAutoDeployEnabled))
	patch.boolean(&body.IsForceHttpsEnabled, inputs.ForceHTTPSEnabled, coolify.Deref(settings.IsForceHttpsEnabled))
	patch.boolean(&body.IsPreviewDeploymentsEnabled, inputs.PreviewDeploymentsEnabled, coolify.Deref(settings.IsPreviewDeploymentsEnabled))
	patch.boolean(&body.HealthCheckEnabled, inputs.HealthCheckEnabled, coolify.Deref(current.HealthCheckEnabled))
	patch.str(&body.HealthCheckPath, inputs.HealthCheckPath, coolify.Deref(current.HealthCheckPath))
	patch.str(&body.HealthCheckPort, inputs.HealthCheckPort, coolify.Deref(current.HealthCheckPort))
	patch.str(&body.HealthCheckMethod, inputs.HealthCheckMethod, coolify.Deref(current.HealthCheckMethod))
	patch.str(&body.LimitsMemory, inputs.LimitsMemory, coolify.Deref(current.LimitsMemory))
	patch.str(&body.LimitsCpus, inputs.LimitsCPUs, coolify.Deref(current.LimitsCpus))
	return body, patch.changed
}

// applicationInputs derives the inputs from the application Coolify reports,
// keeping unmanaged optional inputs and the fields the API does not return
// (identity, source, credentials, create-only settings).
func applicationInputs(previous ApplicationArgs, app api.Application) ApplicationArgs {
	inputs := previous
	settings := coolify.Deref(app.Settings)
	inputs.Name = coolify.Deref(app.Name)
	inputs.Description = coolify.Deref(app.Description)
	inputs.GitRepository = ifSet(previous.GitRepository, coolify.Deref(app.GitRepository))
	inputs.GitBranch = ifSet(previous.GitBranch, coolify.Deref(app.GitBranch))
	inputs.GitCommitSHA = ifSet(previous.GitCommitSHA, coolify.Deref(app.GitCommitSha))
	inputs.BuildPack = ifSet(previous.BuildPack, string(coolify.Deref(app.BuildPack)))
	inputs.DockerRegistryImageName = ifSet(previous.DockerRegistryImageName, coolify.Deref(app.DockerRegistryImageName))
	inputs.DockerRegistryImageTag = ifSet(previous.DockerRegistryImageTag, coolify.Deref(app.DockerRegistryImageTag))
	inputs.Dockerfile = ifSet(previous.Dockerfile, coolify.Deref(app.Dockerfile))
	inputs.DockerfileLocation = ifSet(previous.DockerfileLocation, coolify.Deref(app.DockerfileLocation))
	inputs.Domains = ifSet(previous.Domains, coolify.Deref(app.Fqdn))
	inputs.PortsExposes = ifSet(previous.PortsExposes, coolify.Deref(app.PortsExposes))
	inputs.PortsMappings = ifSet(previous.PortsMappings, coolify.Deref(app.PortsMappings))
	inputs.InstallCommand = ifSet(previous.InstallCommand, coolify.Deref(app.InstallCommand))
	inputs.BuildCommand = ifSet(previous.BuildCommand, coolify.Deref(app.BuildCommand))
	inputs.StartCommand = ifSet(previous.StartCommand, coolify.Deref(app.StartCommand))
	inputs.BaseDirectory = ifSet(previous.BaseDirectory, coolify.Deref(app.BaseDirectory))
	inputs.PublishDirectory = ifSet(previous.PublishDirectory, coolify.Deref(app.PublishDirectory))
	inputs.AutoDeployEnabled = coolify.Deref(settings.IsAutoDeployEnabled)
	inputs.ForceHTTPSEnabled = coolify.Deref(settings.IsForceHttpsEnabled)
	inputs.PreviewDeploymentsEnabled = coolify.Deref(settings.IsPreviewDeploymentsEnabled)
	inputs.HealthCheckEnabled = coolify.Deref(app.HealthCheckEnabled)
	inputs.HealthCheckPath = ifSet(previous.HealthCheckPath, coolify.Deref(app.HealthCheckPath))
	inputs.HealthCheckPort = ifSet(previous.HealthCheckPort, coolify.Deref(app.HealthCheckPort))
	inputs.HealthCheckMethod = ifSet(previous.HealthCheckMethod, coolify.Deref(app.HealthCheckMethod))
	inputs.LimitsMemory = ifSet(previous.LimitsMemory, coolify.Deref(app.LimitsMemory))
	inputs.LimitsCPUs = ifSet(previous.LimitsCPUs, coolify.Deref(app.LimitsCpus))
	return inputs
}

func applicationOwner(uuid string) coolify.Owner {
	return coolify.Owner{Kind: coolify.OwnerApplication, UUID: uuid}
}

func applicationPlacement(args ApplicationArgs) placement {
	return placement{ProjectUUID: args.ProjectUUID, EnvironmentName: args.EnvironmentName}
}

// applicationEnvVars adapts the application environment variable endpoints.
func applicationEnvVars(c *coolify.Client, uuid string) envVars {
	return envVars{
		list: func(ctx context.Context) ([]api.EnvironmentVariable, error) {
			return c.ListApplicationEnvVars(ctx, uuid)
		},
		create: func(ctx context.Context, key, value string) error {
			_, err := c.CreateApplicationEnvVar(ctx, uuid, api.CreateEnvByApplicationUuidJSONRequestBody{
				Key:         coolify.Ptr(key),
				Value:       coolify.Ptr(value),
				IsLiteral:   coolify.Ptr(true),
				IsPreview:   coolify.Ptr(false),
				IsShownOnce: coolify.Ptr(true),
			})
			return err
		},
	}
}

func applicationState(inputs ApplicationArgs, app api.Application) ApplicationState {
	return ApplicationState{
		ApplicationArgs: inputs,
		UUID:            coolify.Deref(app.Uuid),
		FQDN:            coolify.Deref(app.Fqdn),
		Status:          coolify.Deref(app.Status),
	}
}
