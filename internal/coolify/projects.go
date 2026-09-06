package coolify

import (
	"context"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// Environment is a project environment. The OpenAPI specification omits the
// uuid, which the create endpoints accept, so the model is hand-written.
type Environment struct {
	ID        int    `json:"id"`
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	ProjectID int    `json:"project_id"`
}

func (c *Client) ListProjects(ctx context.Context) ([]api.Project, error) {
	return decode[[]api.Project](c.api.ListProjects(ctx))
}

func (c *Client) GetProject(ctx context.Context, uuid string) (api.Project, error) {
	return decode[api.Project](c.api.GetProjectByUuid(ctx, uuid))
}

func (c *Client) CreateProject(ctx context.Context, name, description string) (string, error) {
	return decodeUUID(c.api.CreateProject(ctx, api.CreateProjectJSONRequestBody{
		Name:        &name,
		Description: &description,
	}))
}

func (c *Client) UpdateProject(ctx context.Context, uuid string, body api.UpdateProjectByUuidJSONRequestBody) error {
	return check(c.api.UpdateProjectByUuid(ctx, uuid, body))
}

func (c *Client) DeleteProject(ctx context.Context, uuid string) error {
	return check(c.api.DeleteProjectByUuid(ctx, uuid))
}

func (c *Client) ListEnvironments(ctx context.Context, projectUUID string) ([]Environment, error) {
	return decode[[]Environment](c.api.GetEnvironments(ctx, projectUUID))
}

func (c *Client) GetEnvironment(ctx context.Context, projectUUID, nameOrUUID string) (Environment, error) {
	return decode[Environment](c.api.GetEnvironmentByNameOrUuid(ctx, projectUUID, nameOrUUID))
}

func (c *Client) CreateEnvironment(ctx context.Context, projectUUID, name string) (string, error) {
	return decodeUUID(c.api.CreateEnvironment(ctx, projectUUID, api.CreateEnvironmentJSONRequestBody{Name: &name}))
}

func (c *Client) DeleteEnvironment(ctx context.Context, projectUUID, nameOrUUID string) error {
	return check(c.api.DeleteEnvironment(ctx, projectUUID, nameOrUUID))
}
