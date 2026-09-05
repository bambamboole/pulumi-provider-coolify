package provider

import (
	"context"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// ApplicationSource selects how the application is built.
type ApplicationSource string

const (
	ApplicationSourcePublicGit        ApplicationSource = "public-git"
	ApplicationSourcePrivateDeployKey ApplicationSource = "private-deploy-key"
	ApplicationSourceDockerImage      ApplicationSource = "docker-image"
	ApplicationSourceDockerfile       ApplicationSource = "dockerfile"
)

// Application manages a Coolify application (git, Docker image, or Dockerfile
// source) inside a project environment.
type Application struct{}

type ApplicationArgs struct {
	// Name of the Coolify project the application belongs to.
	Project string `pulumi:"project"`
	// Name of the environment inside the project.
	Environment string `pulumi:"environment"`
	// UUID of the server hosting the application.
	ServerUUID string `pulumi:"serverUuid"`

	// Application source. Exactly one group must be set.
	Source ApplicationSource `pulumi:"source,optional"`
	// Public or private git repository URL.
	GitRepository string `pulumi:"gitRepository,optional"`
	// Git branch to deploy.
	GitBranch string `pulumi:"gitBranch,optional"`
	// Git commit SHA to deploy (optional; defaults to the branch head).
	GitCommitSHA string `pulumi:"gitCommitSha,optional"`
	// Build pack: nixpacks, railpack, static, dockerfile, dockercompose.
	BuildPack string `pulumi:"buildPack,optional"`
	// UUID of the Coolify private key for private-deploy-key sources.
	PrivateKeyUUID string `pulumi:"privateKeyUuid,optional"`
	// GitHub App UUID for private GitHub App sources.
	GitHubAppUUID string `pulumi:"githubAppUuid,optional"`
	// Docker registry image name for Docker image sources.
	DockerRegistryImageName string `pulumi:"dockerRegistryImageName,optional"`
	// Docker registry image tag. Defaults to "latest".
	DockerRegistryImageTag string `pulumi:"dockerRegistryImageTag,optional"`
	// Dockerfile content for Dockerfile sources.
	Dockerfile string `pulumi:"dockerfile,optional"`
	// Location of the Dockerfile inside the repository.
	DockerfileLocation string `pulumi:"dockerfileLocation,optional"`

	// Application name. Defaults to the Pulumi resource name when empty.
	Name string `pulumi:"name,optional"`
	// Description of the application.
	Description string `pulumi:"description,optional"`
	// Exposure and routing.
	Domains       string `pulumi:"domains,optional"`
	PortsExposes  string `pulumi:"portsExposes,optional"`
	PortsMappings string `pulumi:"portsMappings,optional"`
	// Commands and build configuration.
	InstallCommand   string `pulumi:"installCommand,optional"`
	BuildCommand     string `pulumi:"buildCommand,optional"`
	StartCommand     string `pulumi:"startCommand,optional"`
	BaseDirectory    string `pulumi:"baseDirectory,optional"`
	PublishDirectory string `pulumi:"publishDirectory,optional"`
	// Deploy immediately after create.
	InstantDeploy bool `pulumi:"instantDeploy,optional"`
	// Settings.
	AutoDeployEnabled         bool `pulumi:"autoDeployEnabled,optional"`
	ForceHTTPSEnabled         bool `pulumi:"forceHttpsEnabled,optional"`
	PreviewDeploymentsEnabled bool `pulumi:"previewDeploymentsEnabled,optional"`
	// Health checks.
	HealthCheckEnabled bool   `pulumi:"healthCheckEnabled,optional"`
	HealthCheckPath    string `pulumi:"healthCheckPath,optional"`
	HealthCheckPort    string `pulumi:"healthCheckPort,optional"`
	HealthCheckMethod  string `pulumi:"healthCheckMethod,optional"`
	// Resource limits.
	LimitsMemory string `pulumi:"limitsMemory,optional"`
	LimitsCPUs   string `pulumi:"limitsCPUs,optional"`
	// Tags assigned to the application.
	Tags []string `pulumi:"tags,optional"`
}

type ApplicationState struct {
	// UUID of the application in Coolify.
	UUID string `pulumi:"uuid"`
	// Name of the Coolify project.
	Project string `pulumi:"project"`
	// Name of the environment.
	Environment string `pulumi:"environment"`
	// UUID of the hosting server.
	ServerUUID string `pulumi:"serverUuid"`
	// Application source.
	Source ApplicationSource `pulumi:"source"`
	// Git repository URL.
	GitRepository string `pulumi:"gitRepository"`
	// Git branch.
	GitBranch string `pulumi:"gitBranch"`
	// Build pack.
	BuildPack string `pulumi:"buildPack"`
	// Application name.
	Name string `pulumi:"name"`
	// Description of the application.
	Description string `pulumi:"description"`
	// Comma separated domains (FQDN).
	FQDN string `pulumi:"fqdn"`
	// Computed status reported by Coolify.
	Status string `pulumi:"status"`
	// Whether the app is configured to auto-deploy on git push.
	AutoDeployEnabled bool `pulumi:"autoDeployEnabled"`
}

func (r *Application) Annotate(a infer.Annotator) {
	a.SetToken("index", "Application")
	a.Describe(&r, "A Coolify application built from a git repository, a Docker image, or a Dockerfile.")
}

func (state *ApplicationState) Annotate(a infer.Annotator) {
	a.Describe(&state.FQDN, "The FQDN Coolify exposes the application at.")
	a.Describe(&state.Status, "Status reported by Coolify.")
}

func (Application) Create(ctx context.Context, req infer.CreateRequest[ApplicationArgs]) (infer.CreateResponse[ApplicationState], error) {
	if req.DryRun {
		return infer.CreateResponse[ApplicationState]{ID: "pending", Output: applicationPlaceholder(req.Inputs)}, nil
	}
	c := client(ctx)
	state, err := syncApplication(ctx, c, req.Inputs)
	if err != nil {
		return infer.CreateResponse[ApplicationState]{}, err
	}
	return infer.CreateResponse[ApplicationState]{ID: state.UUID, Output: state}, nil
}

func (Application) Diff(ctx context.Context, req infer.DiffRequest[ApplicationArgs, ApplicationState]) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	changed := func(name string, current, desired any) {
		if current != desired {
			diff[name] = p.PropertyDiff{Kind: p.Update}
		}
	}
	changed("project", req.State.Project, req.Inputs.Project)
	changed("environment", req.State.Environment, req.Inputs.Environment)
	changed("serverUuid", req.State.ServerUUID, req.Inputs.ServerUUID)
	changed("source", string(req.State.Source), string(req.Inputs.Source))
	changed("gitRepository", req.State.GitRepository, req.Inputs.GitRepository)
	changed("gitBranch", req.State.GitBranch, req.Inputs.GitBranch)
	changed("name", req.State.Name, effectiveAppName(req.Inputs))
	changed("description", req.State.Description, req.Inputs.Description)
	changed("autoDeployEnabled", req.State.AutoDeployEnabled, req.Inputs.AutoDeployEnabled)
	return infer.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}

func (Application) Update(ctx context.Context, req infer.UpdateRequest[ApplicationArgs, ApplicationState]) (infer.UpdateResponse[ApplicationState], error) {
	if req.DryRun {
		return infer.UpdateResponse[ApplicationState]{
			Output: applicationStateFromRead(req.State.UUID, req.State, req.Inputs),
		}, nil
	}
	c := client(ctx)
	state, err := syncApplication(ctx, c, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[ApplicationState]{}, err
	}
	return infer.UpdateResponse[ApplicationState]{Output: state}, nil
}

func (Application) Read(ctx context.Context, req infer.ReadRequest[ApplicationArgs, ApplicationState]) (infer.ReadResponse[ApplicationArgs, ApplicationState], error) {
	c := client(ctx)
	app, err := c.GetApplication(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[ApplicationArgs, ApplicationState]{}, err
	}
	state, err := normalizeApplication(app, req.Inputs)
	if err != nil {
		return infer.ReadResponse[ApplicationArgs, ApplicationState]{}, err
	}
	return infer.ReadResponse[ApplicationArgs, ApplicationState]{ID: req.ID, Inputs: req.Inputs, State: state}, nil
}

func (Application) Delete(ctx context.Context, req infer.DeleteRequest[ApplicationState]) (infer.DeleteResponse, error) {
	c := client(ctx)
	if err := c.DeleteApplication(ctx, req.State.UUID); err != nil && !NotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func syncApplication(ctx context.Context, c *Client, inputs ApplicationArgs) (ApplicationState, error) {
	identity, err := resolveApplicationIdentity(ctx, c, inputs)
	if err != nil {
		return ApplicationState{}, err
	}

	apps, err := c.ListApplications(ctx)
	if err != nil {
		return ApplicationState{}, err
	}
	var existing *CoolifyApplication
	for i := range apps {
		if apps[i].Name == effectiveAppName(inputs) && apps[i].EnvironmentID == identity.environmentID {
			existing = &apps[i]
			break
		}
	}

	if existing == nil {
		endpoint, body, err := applicationCreateBody(inputs, identity)
		if err != nil {
			return ApplicationState{}, err
		}
		uuid, err := c.CreateApplicationBySource(ctx, endpoint, body)
		if err != nil {
			return ApplicationState{}, err
		}
		app, err := c.GetApplication(ctx, uuid)
		if err != nil {
			return ApplicationState{}, err
		}
		return applicationState(app, identity, inputs), nil
	}

	changes, err := applicationChanges(existing, inputs)
	if err != nil {
		return ApplicationState{}, err
	}
	if len(changes) > 0 {
		if err := c.UpdateApplication(ctx, existing.UUID, changes); err != nil {
			return ApplicationState{}, err
		}
		app, err := c.GetApplication(ctx, existing.UUID)
		if err != nil {
			return ApplicationState{}, err
		}
		return applicationState(app, identity, inputs), nil
	}

	return applicationState(*existing, identity, inputs), nil
}

func resolveApplicationIdentity(ctx context.Context, c *Client, inputs ApplicationArgs) (databaseIdentity, error) {
	return resolveDatabaseIdentity(ctx, c, DatabaseArgs{
		Project:     inputs.Project,
		Environment: inputs.Environment,
		Name:        effectiveAppName(inputs),
	})
}

func effectiveAppName(inputs ApplicationArgs) string {
	if inputs.Name != "" {
		return inputs.Name
	}
	// Fall back to the repo tail so a missing name still produces identity.
	repo := inputs.GitRepository
	for i := len(repo) - 1; i >= 0; i-- {
		if repo[i] == '/' && i+1 < len(repo) {
			return repo[i+1:]
		}
	}
	return repo
}

func applicationCreateBody(inputs ApplicationArgs, identity databaseIdentity) (string, map[string]any, error) {
	body := map[string]any{
		"project_uuid":     identity.projectUUID,
		"server_uuid":      inputs.ServerUUID,
		"environment_name": identity.environment,
	}
	if n := effectiveAppName(inputs); n != "" {
		body["name"] = n
	}
	if inputs.Description != "" {
		body["description"] = inputs.Description
	}
	if inputs.Domains != "" {
		body["domains"] = inputs.Domains
	}
	if inputs.PortsExposes != "" {
		body["ports_exposes"] = inputs.PortsExposes
	}
	if inputs.PortsMappings != "" {
		body["ports_mappings"] = inputs.PortsMappings
	}
	if inputs.InstallCommand != "" {
		body["install_command"] = inputs.InstallCommand
	}
	if inputs.BuildCommand != "" {
		body["build_command"] = inputs.BuildCommand
	}
	if inputs.StartCommand != "" {
		body["start_command"] = inputs.StartCommand
	}
	if inputs.BaseDirectory != "" {
		body["base_directory"] = inputs.BaseDirectory
	}
	if inputs.PublishDirectory != "" {
		body["publish_directory"] = inputs.PublishDirectory
	}
	if inputs.HealthCheckEnabled {
		body["health_check_enabled"] = true
	}
	if inputs.HealthCheckPath != "" {
		body["health_check_path"] = inputs.HealthCheckPath
	}
	if inputs.HealthCheckPort != "" {
		body["health_check_port"] = inputs.HealthCheckPort
	}
	if inputs.HealthCheckMethod != "" {
		body["health_check_method"] = inputs.HealthCheckMethod
	}
	if inputs.LimitsMemory != "" {
		body["limits_memory"] = inputs.LimitsMemory
	}
	if inputs.LimitsCPUs != "" {
		body["limits_cpus"] = inputs.LimitsCPUs
	}
	if len(inputs.Tags) > 0 {
		body["tags"] = inputs.Tags
	}
	if inputs.AutoDeployEnabled {
		body["is_auto_deploy_enabled"] = true
	}
	if inputs.ForceHTTPSEnabled {
		body["is_force_https_enabled"] = true
	}
	if inputs.PreviewDeploymentsEnabled {
		body["is_preview_deployments_enabled"] = true
	}
	settings := applicationSettingsBody(inputs)
	for key, value := range settings {
		body[key] = value
	}
	if inputs.InstantDeploy {
		body["instant_deploy"] = true
	}

	addIfNonEmpty := func(key, value string) {
		if value != "" {
			body[key] = value
		}
	}

	var endpoint string
	switch inputs.Source {
	case ApplicationSourcePublicGit:
		endpoint = "/applications/public"
		body["git_repository"] = inputs.GitRepository
		body["git_branch"] = inputs.GitBranch
		body["build_pack"] = inputs.BuildPack
		addIfNonEmpty("git_commit_sha", inputs.GitCommitSHA)
	case ApplicationSourcePrivateDeployKey:
		endpoint = "/applications/private-deploy-key"
		body["git_repository"] = inputs.GitRepository
		body["git_branch"] = inputs.GitBranch
		body["build_pack"] = inputs.BuildPack
		body["private_key_uuid"] = inputs.PrivateKeyUUID
	case ApplicationSourceDockerImage:
		endpoint = "/applications/dockerimage"
		body["docker_registry_image_name"] = inputs.DockerRegistryImageName
		body["ports_exposes"] = inputs.PortsExposes
		if inputs.DockerRegistryImageTag != "" {
			body["docker_registry_image_tag"] = inputs.DockerRegistryImageTag
		}
	case ApplicationSourceDockerfile:
		endpoint = "/applications/dockerfile"
		body["dockerfile"] = inputs.Dockerfile
		if inputs.DockerfileLocation != "" {
			body["dockerfile_location"] = inputs.DockerfileLocation
		}
	default:
		return "", nil, fmt.Errorf("unsupported application source %q; set source to one of: %s, %s, %s, %s",
			inputs.Source, ApplicationSourcePublicGit, ApplicationSourcePrivateDeployKey, ApplicationSourceDockerImage, ApplicationSourceDockerfile)
	}
	return endpoint, body, nil
}

func applicationSettingsBody(inputs ApplicationArgs) map[string]any {
	out := map[string]any{}
	// The create endpoints accept the settings at the top level under the
	// is_* keys. Nothing extra needed here for the current field set.
	_ = out
	return out
}

func applicationChanges(existing *CoolifyApplication, inputs ApplicationArgs) (map[string]any, error) {
	changes := map[string]any{}
	if existing.Name != effectiveAppName(inputs) {
		changes["name"] = effectiveAppName(inputs)
	}
	if existing.Description != inputs.Description {
		changes["description"] = inputs.Description
	}
	if existing.GitBranch != inputs.GitBranch && inputs.GitBranch != "" {
		changes["git_branch"] = inputs.GitBranch
	}
	if existing.GitRepository != inputs.GitRepository && inputs.GitRepository != "" {
		changes["git_repository"] = inputs.GitRepository
	}
	if existing.PortsExposes != inputs.PortsExposes && inputs.PortsExposes != "" {
		changes["ports_exposes"] = inputs.PortsExposes
	}
	if existing.BuildPack != inputs.BuildPack && inputs.BuildPack != "" {
		changes["build_pack"] = inputs.BuildPack
	}
	return changes, nil
}

func normalizeApplication(app CoolifyApplication, inputs ApplicationArgs) (ApplicationState, error) {
	return applicationState(app, databaseIdentity{}, inputs), nil
}

func applicationState(app CoolifyApplication, identity databaseIdentity, inputs ApplicationArgs) ApplicationState {
	source := inputs.Source
	if inputs.GitRepository != "" && (source == ApplicationSourcePublicGit || source == "") {
		source = ApplicationSourcePublicGit
	}
	return ApplicationState{
		UUID:              app.UUID,
		Project:           inputs.Project,
		Environment:       inputs.Environment,
		ServerUUID:        inputs.ServerUUID,
		Source:            source,
		GitRepository:     app.GitRepository,
		GitBranch:         app.GitBranch,
		BuildPack:         app.BuildPack,
		Name:              app.Name,
		Description:       app.Description,
		FQDN:              app.FQDN,
		Status:            app.Status,
		AutoDeployEnabled: false,
	}
}

func applicationStateFromRead(uuid string, prev ApplicationState, inputs ApplicationArgs) ApplicationState {
	state := applicationState(CoolifyApplication{
		Name: effectiveAppName(inputs),
	}, databaseIdentity{}, inputs)
	state.UUID = uuid
	state.FQDN = prev.FQDN
	state.Status = prev.Status
	state.AutoDeployEnabled = prev.AutoDeployEnabled
	state.GitRepository = ifEmpty(inputs.GitRepository, prev.GitRepository)
	state.GitBranch = ifEmpty(inputs.GitBranch, prev.GitBranch)
	state.BuildPack = ifEmpty(inputs.BuildPack, prev.BuildPack)
	return state
}

func applicationPlaceholder(inputs ApplicationArgs) ApplicationState {
	return applicationState(CoolifyApplication{Name: effectiveAppName(inputs)}, databaseIdentity{}, inputs)
}

func ifEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
