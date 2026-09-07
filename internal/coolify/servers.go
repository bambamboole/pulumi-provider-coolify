package coolify

import (
	"context"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// ServerDetails is a server as returned by GET /servers/{uuid}, which unlike
// the server list carries the numeric ID of the private key the server uses.
// The generated api.Server lacks that field, so it is decoded here.
type ServerDetails struct {
	api.Server
	PrivateKeyID int `json:"private_key_id"`
}

func (c *Client) ListServers(ctx context.Context) ([]api.Server, error) {
	return decode[[]api.Server](c.api.ListServers(ctx))
}

func (c *Client) GetServer(ctx context.Context, uuid string) (api.Server, error) {
	return decode[api.Server](c.api.GetServerByUuid(ctx, uuid))
}

// GetServerDetails fetches a single server including its private key ID.
func (c *Client) GetServerDetails(ctx context.Context, uuid string) (ServerDetails, error) {
	return decode[ServerDetails](c.api.GetServerByUuid(ctx, uuid))
}

func (c *Client) CreateServer(ctx context.Context, body api.CreateServerJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreateServer(ctx, body))
}

func (c *Client) UpdateServer(ctx context.Context, uuid string, body api.UpdateServerByUuidJSONRequestBody) error {
	return check(c.api.UpdateServerByUuid(ctx, uuid, body))
}

func (c *Client) DeleteServer(ctx context.Context, uuid string) error {
	return check(c.api.DeleteServerByUuid(ctx, uuid))
}
