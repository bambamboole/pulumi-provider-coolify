package coolify

import (
	"context"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

func (c *Client) ListApplications(ctx context.Context) ([]api.Application, error) {
	return decode[[]api.Application](c.api.ListApplications(ctx, nil))
}

func (c *Client) GetApplication(ctx context.Context, uuid string) (api.Application, error) {
	return decode[api.Application](c.api.GetApplicationByUuid(ctx, uuid))
}

func (c *Client) CreatePublicApplication(ctx context.Context, body api.CreatePublicApplicationJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreatePublicApplication(ctx, body))
}

func (c *Client) CreatePrivateDeployKeyApplication(ctx context.Context, body api.CreatePrivateDeployKeyApplicationJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreatePrivateDeployKeyApplication(ctx, body))
}

func (c *Client) CreatePrivateGithubAppApplication(ctx context.Context, body api.CreatePrivateGithubAppApplicationJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreatePrivateGithubAppApplication(ctx, body))
}

func (c *Client) CreateDockerfileApplication(ctx context.Context, body api.CreateDockerfileApplicationJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreateDockerfileApplication(ctx, body))
}

func (c *Client) CreateDockerImageApplication(ctx context.Context, body api.CreateDockerimageApplicationJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreateDockerimageApplication(ctx, body))
}

func (c *Client) UpdateApplication(ctx context.Context, uuid string, body api.UpdateApplicationByUuidJSONRequestBody) error {
	return check(c.api.UpdateApplicationByUuid(ctx, uuid, body))
}

func (c *Client) DeleteApplication(ctx context.Context, uuid string) error {
	return check(c.api.DeleteApplicationByUuid(ctx, uuid, nil))
}

// Environment variables

func (c *Client) ListApplicationEnvVars(ctx context.Context, applicationUUID string) ([]api.EnvironmentVariable, error) {
	return decode[[]api.EnvironmentVariable](c.api.ListEnvsByApplicationUuid(ctx, applicationUUID))
}

func (c *Client) CreateApplicationEnvVar(ctx context.Context, applicationUUID string, body api.CreateEnvByApplicationUuidJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreateEnvByApplicationUuid(ctx, applicationUUID, body))
}

func (c *Client) UpdateApplicationEnvVar(ctx context.Context, applicationUUID string, body api.UpdateEnvByApplicationUuidJSONRequestBody) error {
	return check(c.api.UpdateEnvByApplicationUuid(ctx, applicationUUID, body))
}

func (c *Client) DeleteApplicationEnvVar(ctx context.Context, applicationUUID, envUUID string) error {
	return check(c.api.DeleteEnvByApplicationUuid(ctx, applicationUUID, envUUID))
}

// MoveApplication moves the application into another environment, possibly of
// another project. Coolify only re-parents the record; containers keep running.
func (c *Client) MoveApplication(ctx context.Context, uuid, environmentUUID string) error {
	return check(c.api.MoveApplicationByUuid(ctx, uuid, api.MoveApplicationByUuidJSONRequestBody{EnvironmentUuid: environmentUUID}))
}

// ApplicationPlacement is where an application lives: the generated
// Application type only carries the numeric destination and environment IDs,
// while the single-application endpoint nests the destination with its
// server and the environment with its project.
type ApplicationPlacement struct {
	ProjectUUID     string
	EnvironmentName string
	ServerUUID      string
}

type applicationPlacementResponse struct {
	Destination *struct {
		Server *struct {
			Uuid *string `json:"uuid"`
		} `json:"server"`
	} `json:"destination"`
	Environment *struct {
		Name    *string `json:"name"`
		Project *struct {
			Uuid *string `json:"uuid"`
		} `json:"project"`
	} `json:"environment"`
}

// GetApplicationPlacement reads the project, environment and server of an
// application from the nested objects of the single-application endpoint.
func (c *Client) GetApplicationPlacement(ctx context.Context, uuid string) (ApplicationPlacement, error) {
	response, err := decode[applicationPlacementResponse](c.api.GetApplicationByUuid(ctx, uuid))
	if err != nil {
		return ApplicationPlacement{}, err
	}
	var placement ApplicationPlacement
	if response.Destination != nil && response.Destination.Server != nil {
		placement.ServerUUID = Deref(response.Destination.Server.Uuid)
	}
	if response.Environment != nil {
		placement.EnvironmentName = Deref(response.Environment.Name)
		if response.Environment.Project != nil {
			placement.ProjectUUID = Deref(response.Environment.Project.Uuid)
		}
	}
	return placement, nil
}
