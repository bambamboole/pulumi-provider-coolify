package provider

import (
	"context"
	"net/url"
	"strconv"
)

func path(segment string) string {
	return url.PathEscape(segment)
}

// Projects
func (c *Client) ListProjects(ctx context.Context) ([]CoolifyProject, error) {
	var out []CoolifyProject
	return out, c.Do(ctx, "GET", "/projects", nil, &out)
}

func (c *Client) GetProject(ctx context.Context, uuid string) (CoolifyProject, error) {
	var out CoolifyProject
	err := c.Do(ctx, "GET", "/projects/"+path(uuid), nil, &out)
	return out, err
}

func (c *Client) CreateProject(ctx context.Context, name, description string) (string, error) {
	var out struct {
		UUID string `json:"uuid"`
	}
	err := c.Do(ctx, "POST", "/projects", map[string]string{
		"name":        name,
		"description": description,
	}, &out)
	return out.UUID, err
}

func (c *Client) UpdateProject(ctx context.Context, uuid, name, description string) error {
	return c.Do(ctx, "PATCH", "/projects/"+path(uuid), map[string]string{
		"name":        name,
		"description": description,
	}, nil)
}

func (c *Client) DeleteProject(ctx context.Context, uuid string) error {
	return c.Do(ctx, "DELETE", "/projects/"+path(uuid), nil, nil)
}

// Environments
func (c *Client) ListEnvironments(ctx context.Context, projectUUID string) ([]CoolifyEnvironment, error) {
	var out []CoolifyEnvironment
	return out, c.Do(ctx, "GET", "/projects/"+path(projectUUID)+"/environments", nil, &out)
}

func (c *Client) CreateEnvironment(ctx context.Context, projectUUID, name string) error {
	return c.Do(ctx, "POST", "/projects/"+path(projectUUID)+"/environments", map[string]string{"name": name}, nil)
}

func (c *Client) DeleteEnvironment(ctx context.Context, projectUUID, nameOrUUID string) error {
	return c.Do(ctx, "DELETE", "/projects/"+path(projectUUID)+"/environments/"+path(nameOrUUID), nil, nil)
}

// Databases
func (c *Client) ListDatabases(ctx context.Context) ([]CoolifyDatabase, error) {
	var out []CoolifyDatabase
	return out, c.Do(ctx, "GET", "/databases", nil, &out)
}

func (c *Client) GetDatabase(ctx context.Context, uuid string) (CoolifyDatabase, error) {
	var out CoolifyDatabase
	err := c.Do(ctx, "GET", "/databases/"+path(uuid), nil, &out)
	return out, err
}

type CreateDatabaseInput struct {
	ServerUUID      string
	ProjectUUID     string
	EnvironmentName string
	Name            string
	Description     string
	Image           string
	IsPublic        bool
	PublicPort      *int
}

func (c *Client) CreateDatabase(ctx context.Context, typ string, in CreateDatabaseInput) (string, error) {
	body := map[string]any{
		"server_uuid":      in.ServerUUID,
		"project_uuid":     in.ProjectUUID,
		"environment_name": in.EnvironmentName,
		"name":             in.Name,
		"description":      in.Description,
		"image":            in.Image,
		"is_public":        in.IsPublic,
	}
	if in.PublicPort != nil {
		body["public_port"] = *in.PublicPort
	}
	var out struct {
		UUID string `json:"uuid"`
	}
	err := c.Do(ctx, "POST", "/databases/"+path(typ), body, &out)
	return out.UUID, err
}

func (c *Client) UpdateDatabase(ctx context.Context, uuid string, changes map[string]any) error {
	return c.Do(ctx, "PATCH", "/databases/"+path(uuid), changes, nil)
}

func (c *Client) DeleteDatabase(ctx context.Context, uuid string) error {
	return c.Do(ctx, "DELETE", "/databases/"+path(uuid), nil, nil)
}

// Private keys
func (c *Client) ListPrivateKeys(ctx context.Context) ([]CoolifyPrivateKey, error) {
	var out []CoolifyPrivateKey
	return out, c.Do(ctx, "GET", "/security/keys", nil, &out)
}

func (c *Client) GetPrivateKey(ctx context.Context, uuid string) (CoolifyPrivateKey, error) {
	var out CoolifyPrivateKey
	err := c.Do(ctx, "GET", "/security/keys/"+path(uuid), nil, &out)
	return out, err
}

func (c *Client) CreatePrivateKey(ctx context.Context, name, description, privateKey string) (string, error) {
	var out struct {
		UUID string `json:"uuid"`
	}
	err := c.Do(ctx, "POST", "/security/keys", map[string]string{
		"name":        name,
		"description": description,
		"private_key": privateKey,
	}, &out)
	return out.UUID, err
}

// UpdatePrivateKey patches the private key. Like the POST endpoint it requires
// the private_key material to be sent again.
func (c *Client) UpdatePrivateKey(ctx context.Context, name, description, privateKey string) (string, error) {
	var out struct {
		UUID string `json:"uuid"`
	}
	err := c.Do(ctx, "PATCH", "/security/keys", map[string]string{
		"name":        name,
		"description": description,
		"private_key": privateKey,
	}, &out)
	return out.UUID, err
}

func (c *Client) DeletePrivateKey(ctx context.Context, uuid string) error {
	return c.Do(ctx, "DELETE", "/security/keys/"+path(uuid), nil, nil)
}

// Servers
func (c *Client) ListServers(ctx context.Context) ([]CoolifyServer, error) {
	var out []CoolifyServer
	return out, c.Do(ctx, "GET", "/servers", nil, &out)
}

func (c *Client) GetServer(ctx context.Context, uuid string) (CoolifyServer, error) {
	var out CoolifyServer
	err := c.Do(ctx, "GET", "/servers/"+path(uuid), nil, &out)
	return out, err
}

type CreateServerInput struct {
	Name           string
	Description    string
	IP             string
	Port           int
	User           string
	PrivateKeyUUID string
}

func (c *Client) CreateServer(ctx context.Context, in CreateServerInput) (string, error) {
	body := map[string]any{
		"name":             in.Name,
		"description":      in.Description,
		"ip":               in.IP,
		"port":             in.Port,
		"user":             in.User,
		"private_key_uuid": in.PrivateKeyUUID,
		"instant_validate": true,
	}
	var out struct {
		UUID string `json:"uuid"`
	}
	err := c.Do(ctx, "POST", "/servers", body, &out)
	return out.UUID, err
}

func (c *Client) UpdateServer(ctx context.Context, uuid string, changes map[string]any) error {
	return c.Do(ctx, "PATCH", "/servers/"+path(uuid), changes, nil)
}

func (c *Client) DeleteServer(ctx context.Context, uuid string) error {
	return c.Do(ctx, "DELETE", "/servers/"+path(uuid), nil, nil)
}

// S3 Storage
func (c *Client) ListS3Storage(ctx context.Context) ([]CoolifyS3Storage, error) {
	var out []CoolifyS3Storage
	return out, c.Do(ctx, "GET", "/s3-storages", nil, &out)
}

func (c *Client) GetS3Storage(ctx context.Context, uuid string) (CoolifyS3Storage, error) {
	var out CoolifyS3Storage
	err := c.Do(ctx, "GET", "/s3-storages/"+path(uuid), nil, &out)
	return out, err
}

type CreateS3StorageInput struct {
	Name        string
	Description string
	Endpoint    string
	Bucket      string
	Region      string
	AccessKey   string
	SecretKey   string
	IsUsable    bool
}

func (c *Client) CreateS3Storage(ctx context.Context, in CreateS3StorageInput) (string, error) {
	body := map[string]any{
		"name":        in.Name,
		"description": in.Description,
		"endpoint":    in.Endpoint,
		"bucket":      in.Bucket,
		"region":      in.Region,
		"key":         in.AccessKey,
		"secret":      in.SecretKey,
		"is_usable":   in.IsUsable,
	}
	var out struct {
		UUID string `json:"uuid"`
	}
	err := c.Do(ctx, "POST", "/s3-storages", body, &out)
	return out.UUID, err
}

func (c *Client) UpdateS3Storage(ctx context.Context, uuid string, changes map[string]any) error {
	return c.Do(ctx, "PATCH", "/s3-storages/"+path(uuid), changes, nil)
}

func (c *Client) DeleteS3Storage(ctx context.Context, uuid string) error {
	return c.Do(ctx, "DELETE", "/s3-storages/"+path(uuid), nil, nil)
}

// Destinations
func (c *Client) ListServerDestinations(ctx context.Context, serverUUID string) ([]CoolifyDestination, error) {
	var out []CoolifyDestination
	return out, c.Do(ctx, "GET", "/servers/"+path(serverUUID)+"/destinations", nil, &out)
}

// Deployments
func (c *Client) ListDeployments(ctx context.Context) ([]CoolifyDeployment, error) {
	var out []CoolifyDeployment
	return out, c.Do(ctx, "GET", "/deployments", nil, &out)
}

func (c *Client) GetDeployment(ctx context.Context, uuid string) (CoolifyDeployment, error) {
	var out CoolifyDeployment
	err := c.Do(ctx, "GET", "/deployments/"+path(uuid), nil, &out)
	return out, err
}

// DeployOptions configures a deployment trigger.
type DeployOptions struct {
	// Force rebuilds even when there are no new commits.
	Force bool
	// PullRequestID deploys a preview of the given pull request.
	PullRequestID int
	// DockerTag overrides the Docker image tag to deploy.
	DockerTag string
}

// DeployApplication triggers a deployment for the given resource and returns
// the queue items Coolify started.
func (c *Client) DeployApplication(ctx context.Context, uuid string, opts DeployOptions) ([]QueueItem, error) {
	query := url.Values{}
	query.Set("uuid", uuid)
	if opts.Force {
		query.Set("force", "true")
	}
	if opts.PullRequestID > 0 {
		query.Set("pull_request_id", strconv.Itoa(opts.PullRequestID))
	}
	if opts.DockerTag != "" {
		query.Set("docker_tag", opts.DockerTag)
	}
	var out struct {
		Deployments []QueueItem `json:"deployments"`
	}
	err := c.Do(ctx, "POST", "/deploy?"+query.Encode(), map[string]any{}, &out)
	return out.Deployments, err
}

type QueueItem struct {
	Message        string `json:"message"`
	ResourceUUID   string `json:"resource_uuid"`
	DeploymentUUID string `json:"deployment_uuid"`
}

// Applications
func (c *Client) ListApplications(ctx context.Context) ([]CoolifyApplication, error) {
	var out []CoolifyApplication
	return out, c.Do(ctx, "GET", "/applications", nil, &out)
}

func (c *Client) GetApplication(ctx context.Context, uuid string) (CoolifyApplication, error) {
	var out CoolifyApplication
	err := c.Do(ctx, "GET", "/applications/"+path(uuid), nil, &out)
	return out, err
}

// CreateApplicationBySource sends a create request to the given application
// endpoint (e.g. /applications/public) with the built body.
func (c *Client) CreateApplicationBySource(ctx context.Context, endpoint string, body map[string]any) (string, error) {
	var out struct {
		UUID string `json:"uuid"`
	}
	err := c.Do(ctx, "POST", endpoint, body, &out)
	return out.UUID, err
}

func (c *Client) UpdateApplication(ctx context.Context, uuid string, changes map[string]any) error {
	return c.Do(ctx, "PATCH", "/applications/"+path(uuid), changes, nil)
}

func (c *Client) DeleteApplication(ctx context.Context, uuid string) error {
	return c.Do(ctx, "DELETE", "/applications/"+path(uuid), nil, nil)
}

// Application environment variables
func (c *Client) ListApplicationEnvVars(ctx context.Context, uuid string) ([]CoolifyEnvironmentVariable, error) {
	var out []CoolifyEnvironmentVariable
	return out, c.Do(ctx, "GET", "/applications/"+path(uuid)+"/envs", nil, &out)
}

// CreateApplicationEnvVarInput describes an environment variable to create or
// patch on an application.
type CreateApplicationEnvVarInput struct {
	Key         string
	Value       string
	IsPreview   bool
	IsShownOnce bool
}

func (c *Client) CreateApplicationEnvVar(ctx context.Context, uuid string, in CreateApplicationEnvVarInput) (string, error) {
	var out struct {
		UUID string `json:"uuid"`
	}
	err := c.Do(ctx, "POST", "/applications/"+path(uuid)+"/envs", applicationEnvVarBody(in), &out)
	return out.UUID, err
}

func (c *Client) UpdateApplicationEnvVar(ctx context.Context, uuid, envUUID string, in CreateApplicationEnvVarInput) error {
	return c.Do(ctx, "PATCH", "/applications/"+path(uuid)+"/envs/"+path(envUUID), applicationEnvVarBody(in), nil)
}

func (c *Client) DeleteApplicationEnvVar(ctx context.Context, uuid, envUUID string) error {
	return c.Do(ctx, "DELETE", "/applications/"+path(uuid)+"/envs/"+path(envUUID), nil, nil)
}

func applicationEnvVarBody(in CreateApplicationEnvVarInput) map[string]any {
	return map[string]any{
		"key":           in.Key,
		"value":         in.Value,
		"is_literal":    true,
		"is_preview":    in.IsPreview,
		"is_shown_once": in.IsShownOnce,
	}
}
