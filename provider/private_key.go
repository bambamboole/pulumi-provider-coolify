package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// PrivateKey manages a Coolify SSH private key. Coolify's API cannot update a
// key after creation, so changes to the name or key material replace it and a
// changed description is only applied when the key is (re)created.
type PrivateKey struct{}

type PrivateKeyArgs struct {
	// Name of the private key. An existing key with this name is adopted.
	Name string `pulumi:"name"`
	// Description of the private key. Applied on create only.
	Description string `pulumi:"description,optional"`
	// PEM encoded private key material. Required to create a key that does not
	// exist yet; adopted keys can be managed without it.
	PrivateKey string `pulumi:"privateKey,optional" provider:"secret"`
}

type PrivateKeyState struct {
	PrivateKeyArgs
	// UUID of the private key in Coolify.
	UUID string `pulumi:"uuid"`
	// Public key derived from the private key.
	PublicKey string `pulumi:"publicKey"`
	// Fingerprint of the key.
	Fingerprint string `pulumi:"fingerprint"`
}

func (r *PrivateKey) Annotate(a infer.Annotator) {
	a.SetToken("index", "PrivateKey")
	a.Describe(&r, "A Coolify SSH private key. An existing key with the same name is adopted on create. Coolify's API cannot update keys, so changing the name or key material replaces the key.")
}

func (args *PrivateKeyArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "Name of the private key. An existing key with this name is adopted. Changing it replaces the key.")
	a.Describe(&args.Description, "Description of the private key. Applied on create only, because Coolify's API cannot update keys.")
	a.Describe(&args.PrivateKey, "PEM encoded private key material. Required to create a key that does not exist yet; adopted keys can be managed without it. Changing it replaces the key.")
}

func (state *PrivateKeyState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the private key in Coolify.")
	a.Describe(&state.PublicKey, "Public key derived from the private key.")
	a.Describe(&state.Fingerprint, "Fingerprint of the key.")
}

func (PrivateKey) Create(ctx context.Context, req infer.CreateRequest[PrivateKeyArgs]) (infer.CreateResponse[PrivateKeyState], error) {
	if req.DryRun {
		return infer.CreateResponse[PrivateKeyState]{Output: PrivateKeyState{PrivateKeyArgs: req.Inputs}}, nil
	}
	key, err := createPrivateKey(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.CreateResponse[PrivateKeyState]{}, err
	}
	return infer.CreateResponse[PrivateKeyState]{ID: coolify.Deref(key.Uuid), Output: privateKeyState(req.Inputs, key)}, nil
}

func (PrivateKey) Diff(ctx context.Context, req infer.DiffRequest[PrivateKeyArgs, PrivateKeyState]) (infer.DiffResponse, error) {
	diff := diffArgs(req.State.PrivateKeyArgs, req.Inputs, "name", "privateKey")
	// Coolify cannot update keys; the description is only applied on create.
	delete(diff, "description")
	return diffResponse(diff, req.State.Name == req.Inputs.Name), nil
}

func (PrivateKey) Read(ctx context.Context, req infer.ReadRequest[PrivateKeyArgs, PrivateKeyState]) (infer.ReadResponse[PrivateKeyArgs, PrivateKeyState], error) {
	key, err := client(ctx).GetPrivateKey(ctx, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[PrivateKeyArgs, PrivateKeyState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[PrivateKeyArgs, PrivateKeyState]{}, err
	}
	inputs := req.Inputs
	inputs.Name = coolify.Deref(key.Name)
	inputs.Description = coolify.Deref(key.Description)
	return infer.ReadResponse[PrivateKeyArgs, PrivateKeyState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  privateKeyState(inputs, key),
	}, nil
}

func (PrivateKey) Delete(ctx context.Context, req infer.DeleteRequest[PrivateKeyState]) (infer.DeleteResponse, error) {
	if err := client(ctx).DeletePrivateKey(ctx, req.ID); err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

// createPrivateKey adopts the key with the given name or creates it from the
// provided material.
func createPrivateKey(ctx context.Context, c *coolify.Client, inputs PrivateKeyArgs) (api.PrivateKey, error) {
	keys, err := c.ListPrivateKeys(ctx)
	if err != nil {
		return api.PrivateKey{}, err
	}
	for _, key := range keys {
		if coolify.Deref(key.Name) != inputs.Name {
			continue
		}
		if existing := strings.TrimSpace(coolify.Deref(key.PrivateKey)); existing != "" && inputs.PrivateKey != "" &&
			existing != strings.TrimSpace(inputs.PrivateKey) {
			return api.PrivateKey{}, fmt.Errorf("coolify private key %q already exists with different key material; choose another name to create a new key", inputs.Name)
		}
		return key, nil
	}
	if inputs.PrivateKey == "" {
		return api.PrivateKey{}, fmt.Errorf("coolify private key %q does not exist and no privateKey was provided to create it", inputs.Name)
	}
	uuid, err := c.CreatePrivateKey(ctx, inputs.Name, inputs.Description, inputs.PrivateKey)
	if err != nil {
		return api.PrivateKey{}, err
	}
	return c.GetPrivateKey(ctx, uuid)
}

func privateKeyState(inputs PrivateKeyArgs, key api.PrivateKey) PrivateKeyState {
	return PrivateKeyState{
		PrivateKeyArgs: inputs,
		UUID:           coolify.Deref(key.Uuid),
		PublicKey:      coolify.Deref(key.PublicKey),
		Fingerprint:    coolify.Deref(key.Fingerprint),
	}
}
