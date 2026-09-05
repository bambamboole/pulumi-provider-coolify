package provider

import (
	"context"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// PrivateKey manages a Coolify SSH private key. The key material is only sent
// when the resource is created or the inputs change; adopted keys are never
// patched, because the Coolify API requires resending the material.
type PrivateKey struct{}

type PrivateKeyArgs struct {
	// Name of the private key.
	Name string `pulumi:"name"`
	// Description of the private key.
	Description string `pulumi:"description,optional"`
	// PEM encoded private key material. Required to create a key that does not
	// exist yet. Never stored in state; adopted keys stay read-only.
	PrivateKey string `pulumi:"privateKey,optional,provider:secret"`
}

type PrivateKeyState struct {
	// UUID of the private key in Coolify.
	UUID string `pulumi:"uuid"`
	// Name of the private key.
	Name string `pulumi:"name"`
	// Description of the private key.
	Description string `pulumi:"description"`
	// Public key derived from the private key.
	PublicKey *string `pulumi:"publicKey,optional"`
}

func (r *PrivateKey) Annotate(a infer.Annotator) {
	a.SetToken("index", "PrivateKey")
	a.Describe(&r, "A Coolify SSH private key.")
}

func (args *PrivateKeyArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.PrivateKey, "PEM private key material. Required to create a missing key; never stored in state.")
}

func (state *PrivateKeyState) Annotate(a infer.Annotator) {
	a.Describe(&state.PublicKey, "Public key derived from the private key.")
}

func (PrivateKey) Create(ctx context.Context, req infer.CreateRequest[PrivateKeyArgs]) (infer.CreateResponse[PrivateKeyState], error) {
	if req.DryRun {
		return infer.CreateResponse[PrivateKeyState]{ID: "pending", Output: privateKeyPlaceholder(req.Inputs)}, nil
	}
	c := client(ctx)
	state, err := syncPrivateKey(ctx, c, req.Inputs)
	if err != nil {
		return infer.CreateResponse[PrivateKeyState]{}, err
	}
	return infer.CreateResponse[PrivateKeyState]{ID: state.UUID, Output: state}, nil
}

func (PrivateKey) Diff(ctx context.Context, req infer.DiffRequest[PrivateKeyArgs, PrivateKeyState]) (infer.DiffResponse, error) {
	if req.Inputs.PrivateKey == "" {
		// No key material, adopted key: keep it read-only.
		return infer.DiffResponse{HasChanges: false}, nil
	}
	diff := map[string]p.PropertyDiff{}
	if req.Inputs.Name != req.State.Name {
		diff["name"] = p.PropertyDiff{Kind: p.Update}
	}
	if req.Inputs.Description != req.State.Description {
		diff["description"] = p.PropertyDiff{Kind: p.Update}
	}
	return infer.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}

func (PrivateKey) Update(ctx context.Context, req infer.UpdateRequest[PrivateKeyArgs, PrivateKeyState]) (infer.UpdateResponse[PrivateKeyState], error) {
	if req.DryRun {
		return infer.UpdateResponse[PrivateKeyState]{Output: privateKeyStateFromInputs(req.State.UUID, req.Inputs)}, nil
	}
	c := client(ctx)
	state, err := syncPrivateKey(ctx, c, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[PrivateKeyState]{}, err
	}
	return infer.UpdateResponse[PrivateKeyState]{Output: state}, nil
}

func (PrivateKey) Read(ctx context.Context, req infer.ReadRequest[PrivateKeyArgs, PrivateKeyState]) (infer.ReadResponse[PrivateKeyArgs, PrivateKeyState], error) {
	c := client(ctx)
	key, err := c.GetPrivateKey(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[PrivateKeyArgs, PrivateKeyState]{}, err
	}
	inputs := req.Inputs
	if inputs.Name == "" {
		inputs.Name = key.Name
		inputs.Description = desc(key.Description)
	}
	return infer.ReadResponse[PrivateKeyArgs, PrivateKeyState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  privateKeyState(key),
	}, nil
}

func (PrivateKey) Delete(ctx context.Context, req infer.DeleteRequest[PrivateKeyState]) (infer.DeleteResponse, error) {
	c := client(ctx)
	if err := c.DeletePrivateKey(ctx, req.State.UUID); err != nil && !NotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func syncPrivateKey(ctx context.Context, c *Client, inputs PrivateKeyArgs) (PrivateKeyState, error) {

	keys, err := c.ListPrivateKeys(ctx)
	if err != nil {
		return PrivateKeyState{}, err
	}
	var existing *CoolifyPrivateKey
	for i := range keys {
		if keys[i].Name == inputs.Name {
			existing = &keys[i]
			break
		}
	}

	if existing == nil {
		if inputs.PrivateKey == "" {
			return PrivateKeyState{}, fmt.Errorf(`coolify private key %q does not exist and no privateKey was provided to create it`, inputs.Name)
		}
		uuid, err := c.CreatePrivateKey(ctx, inputs.Name, inputs.Description, inputs.PrivateKey)
		if err != nil {
			return PrivateKeyState{}, err
		}
		key, err := c.GetPrivateKey(ctx, uuid)
		if err != nil {
			return PrivateKeyState{}, err
		}
		return privateKeyState(key), nil
	}

	if inputs.PrivateKey != "" &&
		(existing.Name != inputs.Name || desc(existing.Description) != inputs.Description) {
		uuid, err := c.UpdatePrivateKey(ctx, inputs.Name, inputs.Description, inputs.PrivateKey)
		if err != nil {
			return PrivateKeyState{}, err
		}
		key, err := c.GetPrivateKey(ctx, uuid)
		if err != nil {
			return PrivateKeyState{}, err
		}
		return privateKeyState(key), nil
	}

	return privateKeyState(*existing), nil
}

func privateKeyState(key CoolifyPrivateKey) PrivateKeyState {
	return PrivateKeyState{
		UUID:        key.UUID,
		Name:        key.Name,
		Description: desc(key.Description),
		PublicKey:   key.PublicKey,
	}
}

func privateKeyStateFromInputs(uuid string, inputs PrivateKeyArgs) PrivateKeyState {
	return PrivateKeyState{UUID: uuid, Name: inputs.Name, Description: inputs.Description}
}

func privateKeyPlaceholder(inputs PrivateKeyArgs) PrivateKeyState {
	state := privateKeyStateFromInputs("pending", inputs)
	return state
}
