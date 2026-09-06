package coolify

import (
	"context"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

func (c *Client) ListServices(ctx context.Context) ([]api.Service, error) {
	return decode[[]api.Service](c.api.ListServices(ctx))
}

func (c *Client) GetService(ctx context.Context, uuid string) (api.Service, error) {
	return decode[api.Service](c.api.GetServiceByUuid(ctx, uuid))
}

// CreateService creates a one-click or docker-compose service and returns its UUID.
func (c *Client) CreateService(ctx context.Context, body api.CreateServiceJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreateService(ctx, body))
}

func (c *Client) UpdateService(ctx context.Context, uuid string, body api.UpdateServiceByUuidJSONRequestBody) error {
	return check(c.api.UpdateServiceByUuid(ctx, uuid, body))
}

func (c *Client) DeleteService(ctx context.Context, uuid string) error {
	return check(c.api.DeleteServiceByUuid(ctx, uuid, nil))
}

// MoveService moves the service into another environment, possibly of another
// project. Coolify only re-parents the record; containers keep running.
func (c *Client) MoveService(ctx context.Context, uuid, environmentUUID string) error {
	return check(c.api.MoveServiceByUuid(ctx, uuid, api.MoveServiceByUuidJSONRequestBody{EnvironmentUuid: environmentUUID}))
}

// Environment variables

func (c *Client) ListServiceEnvVars(ctx context.Context, serviceUUID string) ([]api.EnvironmentVariable, error) {
	return decode[[]api.EnvironmentVariable](c.api.ListEnvsByServiceUuid(ctx, serviceUUID))
}

func (c *Client) CreateServiceEnvVar(ctx context.Context, serviceUUID string, body api.CreateEnvByServiceUuidJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreateEnvByServiceUuid(ctx, serviceUUID, body))
}
