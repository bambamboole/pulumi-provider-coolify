package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// S3Storage manages a Coolify S3-compatible storage destination, e.g. an R2
// bucket used for backups.
type S3Storage struct{}

type S3StorageArgs struct {
	// Name of the storage. An existing storage with this name is adopted.
	Name string `pulumi:"name"`
	// Description of the storage.
	Description string `pulumi:"description,optional"`
	// S3 endpoint URL, e.g. https://<account>.eu.r2.cloudflarestorage.com.
	Endpoint string `pulumi:"endpoint"`
	// S3 bucket name.
	Bucket string `pulumi:"bucket"`
	// S3 region ("auto" for R2).
	Region string `pulumi:"region,optional"`
	// S3 access key.
	AccessKey string `pulumi:"accessKey" provider:"secret"`
	// S3 secret key.
	SecretKey string `pulumi:"secretKey" provider:"secret"`
}

type S3StorageState struct {
	S3StorageArgs
	// UUID of the storage in Coolify.
	UUID string `pulumi:"uuid"`
	// Whether Coolify validated the storage as usable.
	IsUsable bool `pulumi:"isUsable"`
}

func (r *S3Storage) Annotate(a infer.Annotator) {
	a.SetToken("index", "S3Storage")
	a.Describe(&r, "A Coolify S3-compatible storage destination, e.g. an R2 bucket for backups. An existing storage with the same name is adopted on create.")
}

func (args *S3StorageArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "Name of the storage. An existing storage with this name is adopted.")
	a.Describe(&args.Description, "Description of the storage.")
	a.Describe(&args.Endpoint, "S3 endpoint URL, e.g. https://<account>.eu.r2.cloudflarestorage.com.")
	a.Describe(&args.Bucket, "S3 bucket name.")
	a.Describe(&args.Region, `S3 region ("auto" for R2).`)
	a.Describe(&args.AccessKey, "S3 access key.")
	a.Describe(&args.SecretKey, "S3 secret key.")
}

func (state *S3StorageState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the storage in Coolify.")
	a.Describe(&state.IsUsable, "Whether Coolify validated the storage as usable.")
}

func (S3Storage) Create(ctx context.Context, req infer.CreateRequest[S3StorageArgs]) (infer.CreateResponse[S3StorageState], error) {
	if req.DryRun {
		return infer.CreateResponse[S3StorageState]{Output: S3StorageState{S3StorageArgs: req.Inputs}}, nil
	}
	storage, err := createS3Storage(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.CreateResponse[S3StorageState]{}, err
	}
	return infer.CreateResponse[S3StorageState]{ID: storage.UUID, Output: s3StorageState(req.Inputs, storage)}, nil
}

func (S3Storage) Diff(ctx context.Context, req infer.DiffRequest[S3StorageArgs, S3StorageState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.S3StorageArgs, req.Inputs), true), nil
}

func (S3Storage) Update(ctx context.Context, req infer.UpdateRequest[S3StorageArgs, S3StorageState]) (infer.UpdateResponse[S3StorageState], error) {
	if req.DryRun {
		return infer.UpdateResponse[S3StorageState]{Output: S3StorageState{S3StorageArgs: req.Inputs, UUID: req.ID, IsUsable: req.State.IsUsable}}, nil
	}
	c := client(ctx)
	current, err := c.GetS3Storage(ctx, req.ID)
	if err != nil {
		return infer.UpdateResponse[S3StorageState]{}, err
	}
	storage, err := applyS3Storage(ctx, c, current, req.State.S3StorageArgs, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[S3StorageState]{}, err
	}
	return infer.UpdateResponse[S3StorageState]{Output: s3StorageState(req.Inputs, storage)}, nil
}

func (S3Storage) Read(ctx context.Context, req infer.ReadRequest[S3StorageArgs, S3StorageState]) (infer.ReadResponse[S3StorageArgs, S3StorageState], error) {
	storage, err := client(ctx).GetS3Storage(ctx, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[S3StorageArgs, S3StorageState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[S3StorageArgs, S3StorageState]{}, err
	}
	// The API does not return the credentials, so those inputs are kept as is.
	inputs := req.Inputs
	inputs.Name = storage.Name
	inputs.Description = coolify.Deref(storage.Description)
	inputs.Endpoint = storage.Endpoint
	inputs.Bucket = storage.Bucket
	inputs.Region = ifSet(req.Inputs.Region, storage.Region)
	return infer.ReadResponse[S3StorageArgs, S3StorageState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  s3StorageState(inputs, storage),
	}, nil
}

func (S3Storage) Delete(ctx context.Context, req infer.DeleteRequest[S3StorageState]) (infer.DeleteResponse, error) {
	if err := client(ctx).DeleteS3Storage(ctx, req.ID); err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

// createS3Storage adopts the storage with the given name or creates it.
func createS3Storage(ctx context.Context, c *coolify.Client, inputs S3StorageArgs) (coolify.S3Storage, error) {
	storages, err := c.ListS3Storages(ctx)
	if err != nil {
		return coolify.S3Storage{}, err
	}
	for _, storage := range storages {
		if storage.Name == inputs.Name {
			// Credentials are not readable, so they are always re-applied on adopt.
			return applyS3Storage(ctx, c, storage, S3StorageArgs{}, inputs)
		}
	}
	uuid, err := c.CreateS3Storage(ctx, api.CreateS3StorageJSONRequestBody{
		Name:        inputs.Name,
		Description: &inputs.Description,
		Endpoint:    inputs.Endpoint,
		Bucket:      inputs.Bucket,
		Region:      inputs.Region,
		Key:         inputs.AccessKey,
		Secret:      inputs.SecretKey,
		IsUsable:    coolify.Ptr(true),
	})
	if err != nil {
		return coolify.S3Storage{}, err
	}
	return c.GetS3Storage(ctx, uuid)
}

// applyS3Storage patches the fields of current that differ from the inputs.
// Credentials are compared against the previous inputs because the API omits them.
func applyS3Storage(ctx context.Context, c *coolify.Client, current coolify.S3Storage, previous, inputs S3StorageArgs) (coolify.S3Storage, error) {
	var body api.UpdateS3StorageByUuidJSONRequestBody
	var patch patch
	patch.str(&body.Name, inputs.Name, current.Name)
	patch.text(&body.Description, inputs.Description, coolify.Deref(current.Description))
	patch.str(&body.Endpoint, inputs.Endpoint, current.Endpoint)
	patch.str(&body.Bucket, inputs.Bucket, current.Bucket)
	patch.str(&body.Region, inputs.Region, current.Region)
	patch.str(&body.Key, inputs.AccessKey, previous.AccessKey)
	patch.str(&body.Secret, inputs.SecretKey, previous.SecretKey)
	if !patch.changed {
		return current, nil
	}
	if err := c.UpdateS3Storage(ctx, current.UUID, body); err != nil {
		return coolify.S3Storage{}, err
	}
	return c.GetS3Storage(ctx, current.UUID)
}

func s3StorageState(inputs S3StorageArgs, storage coolify.S3Storage) S3StorageState {
	return S3StorageState{S3StorageArgs: inputs, UUID: storage.UUID, IsUsable: storage.IsUsable}
}
