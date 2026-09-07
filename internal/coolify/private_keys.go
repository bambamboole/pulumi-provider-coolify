package coolify

import (
	"context"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

func (c *Client) ListPrivateKeys(ctx context.Context) ([]api.PrivateKey, error) {
	return decode[[]api.PrivateKey](c.api.ListPrivateKeys(ctx))
}

func (c *Client) GetPrivateKey(ctx context.Context, uuid string) (api.PrivateKey, error) {
	return decode[api.PrivateKey](c.api.GetPrivateKeyByUuid(ctx, uuid))
}

func (c *Client) CreatePrivateKey(ctx context.Context, name, description, privateKey string) (string, error) {
	return decodeUUID(c.api.CreatePrivateKey(ctx, api.CreatePrivateKeyJSONRequestBody{
		Name:        &name,
		Description: &description,
		PrivateKey:  privateKey,
	}))
}

// UpdatePrivateKey updates the key Coolify matches to the given material. The
// endpoint has no UUID parameter and requires the private key to be resent.
func (c *Client) UpdatePrivateKey(ctx context.Context, name, description, privateKey string) (string, error) {
	return decodeUUID(c.api.UpdatePrivateKey(ctx, api.UpdatePrivateKeyJSONRequestBody{
		Name:        &name,
		Description: &description,
		PrivateKey:  privateKey,
	}))
}

func (c *Client) DeletePrivateKey(ctx context.Context, uuid string) error {
	return check(c.api.DeletePrivateKeyByUuid(ctx, uuid))
}

// PrivateKeyUUIDByID maps the numeric ID servers and GitHub Apps reference to
// the key's UUID. It returns an empty string when the ID is zero or no key
// carries it.
func (c *Client) PrivateKeyUUIDByID(ctx context.Context, id int) (string, error) {
	if id == 0 {
		return "", nil
	}
	keys, err := c.ListPrivateKeys(ctx)
	if err != nil {
		return "", err
	}
	for _, key := range keys {
		if key.Id != nil && *key.Id == id {
			return Deref(key.Uuid), nil
		}
	}
	return "", nil
}
