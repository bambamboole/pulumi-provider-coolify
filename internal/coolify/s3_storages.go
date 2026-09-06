package coolify

import (
	"context"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// S3Storage is an S3-compatible storage destination. The OpenAPI specification
// only describes it inline, so the model is hand-written.
type S3Storage struct {
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Endpoint    string  `json:"endpoint"`
	Bucket      string  `json:"bucket"`
	Region      string  `json:"region"`
	IsUsable    bool    `json:"is_usable"`
}

func (c *Client) ListS3Storages(ctx context.Context) ([]S3Storage, error) {
	return decode[[]S3Storage](c.api.ListS3Storages(ctx))
}

func (c *Client) GetS3Storage(ctx context.Context, uuid string) (S3Storage, error) {
	return decode[S3Storage](c.api.GetS3StorageByUuid(ctx, uuid))
}

func (c *Client) CreateS3Storage(ctx context.Context, body api.CreateS3StorageJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreateS3Storage(ctx, body))
}

func (c *Client) UpdateS3Storage(ctx context.Context, uuid string, body api.UpdateS3StorageByUuidJSONRequestBody) error {
	return check(c.api.UpdateS3StorageByUuid(ctx, uuid, body))
}

func (c *Client) DeleteS3Storage(ctx context.Context, uuid string) error {
	return check(c.api.DeleteS3StorageByUuid(ctx, uuid))
}
