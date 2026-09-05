package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// S3Storage manages a Coolify S3-compatible storage destination (e.g. an R2
// bucket). The access key and secret key are only sent on create and when the
// input fields change; they are never stored in state.
type S3Storage struct{}

type S3StorageArgs struct {
	// Name of the storage.
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
	AccessKey string `pulumi:"accessKey,provider:secret"`
	// S3 secret key.
	SecretKey string `pulumi:"secretKey,provider:secret"`
}

type S3StorageState struct {
	// UUID of the storage in Coolify.
	UUID string `pulumi:"uuid"`
	// Name of the storage.
	Name string `pulumi:"name"`
	// Description of the storage.
	Description string `pulumi:"description"`
	// S3 endpoint URL.
	Endpoint string `pulumi:"endpoint"`
	// S3 bucket name.
	Bucket string `pulumi:"bucket"`
	// S3 region.
	Region string `pulumi:"region"`
	// Whether Coolify validated the storage as usable.
	IsUsable bool `pulumi:"isUsable"`
}

func (r *S3Storage) Annotate(a infer.Annotator) {
	a.SetToken("index", "S3Storage")
	a.Describe(&r, "A Coolify S3-compatible storage destination, e.g. an R2 bucket for backups.")
}

func (args *S3StorageArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.AccessKey, "S3 access key. Never stored in state.")
	a.Describe(&args.SecretKey, "S3 secret key. Never stored in state.")
}

func (state *S3StorageState) Annotate(a infer.Annotator) {
	a.Describe(&state.IsUsable, "Whether Coolify validated the storage as usable.")
}

func (S3Storage) Create(ctx context.Context, req infer.CreateRequest[S3StorageArgs]) (infer.CreateResponse[S3StorageState], error) {
	if req.DryRun {
		return infer.CreateResponse[S3StorageState]{ID: "pending", Output: s3Placeholder(req.Inputs)}, nil
	}
	c := client(ctx)
	state, err := syncS3Storage(ctx, c, req.Inputs)
	if err != nil {
		return infer.CreateResponse[S3StorageState]{}, err
	}
	return infer.CreateResponse[S3StorageState]{ID: state.UUID, Output: state}, nil
}

func (S3Storage) Update(ctx context.Context, req infer.UpdateRequest[S3StorageArgs, S3StorageState]) (infer.UpdateResponse[S3StorageState], error) {
	if req.DryRun {
		return infer.UpdateResponse[S3StorageState]{
			Output: s3State(req.State.UUID, req.Inputs, req.State.IsUsable),
		}, nil
	}
	c := client(ctx)
	state, err := syncS3Storage(ctx, c, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[S3StorageState]{}, err
	}
	return infer.UpdateResponse[S3StorageState]{Output: state}, nil
}

func (S3Storage) Read(ctx context.Context, req infer.ReadRequest[S3StorageArgs, S3StorageState]) (infer.ReadResponse[S3StorageArgs, S3StorageState], error) {
	c := client(ctx)
	storage, err := c.GetS3Storage(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[S3StorageArgs, S3StorageState]{}, err
	}
	inputs := req.Inputs
	if inputs.Name == "" {
		inputs.Name = storage.Name
		inputs.Description = desc(storage.Description)
		inputs.Endpoint = storage.Endpoint
		inputs.Bucket = storage.Bucket
		inputs.Region = storage.Region
	}
	return infer.ReadResponse[S3StorageArgs, S3StorageState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  normalizeS3State(storage),
	}, nil
}

func (S3Storage) Delete(ctx context.Context, req infer.DeleteRequest[S3StorageState]) (infer.DeleteResponse, error) {
	c := client(ctx)
	if err := c.DeleteS3Storage(ctx, req.State.UUID); err != nil && !NotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func syncS3Storage(ctx context.Context, c *Client, inputs S3StorageArgs) (S3StorageState, error) {

	storages, err := c.ListS3Storage(ctx)
	if err != nil {
		return S3StorageState{}, err
	}
	var existing *CoolifyS3Storage
	for i := range storages {
		if storages[i].Name == inputs.Name {
			existing = &storages[i]
			break
		}
	}

	if existing == nil {
		uuid, err := c.CreateS3Storage(ctx, CreateS3StorageInput{
			Name:        inputs.Name,
			Description: inputs.Description,
			Endpoint:    inputs.Endpoint,
			Bucket:      inputs.Bucket,
			Region:      inputs.Region,
			AccessKey:   inputs.AccessKey,
			SecretKey:   inputs.SecretKey,
			IsUsable:    true,
		})
		if err != nil {
			return S3StorageState{}, err
		}
		storage, err := c.GetS3Storage(ctx, uuid)
		if err != nil {
			return S3StorageState{}, err
		}
		return normalizeS3State(storage), nil
	}

	changes := map[string]any{}
	if existing.Name != inputs.Name {
		changes["name"] = inputs.Name
	}
	if desc(existing.Description) != inputs.Description {
		changes["description"] = inputs.Description
	}
	if existing.Endpoint != inputs.Endpoint {
		changes["endpoint"] = inputs.Endpoint
	}
	if existing.Bucket != inputs.Bucket {
		changes["bucket"] = inputs.Bucket
	}
	if existing.Region != inputs.Region {
		changes["region"] = inputs.Region
	}
	if len(changes) > 0 {
		if err := c.UpdateS3Storage(ctx, existing.UUID, changes); err != nil {
			return S3StorageState{}, err
		}
		storage, err := c.GetS3Storage(ctx, existing.UUID)
		if err != nil {
			return S3StorageState{}, err
		}
		return normalizeS3State(storage), nil
	}

	return normalizeS3State(*existing), nil
}

func normalizeS3State(storage CoolifyS3Storage) S3StorageState {
	return S3StorageState{
		UUID:        storage.UUID,
		Name:        storage.Name,
		Description: desc(storage.Description),
		Endpoint:    storage.Endpoint,
		Bucket:      storage.Bucket,
		Region:      storage.Region,
		IsUsable:    storage.IsUsable,
	}
}

func s3State(uuid string, inputs S3StorageArgs, isUsable bool) S3StorageState {
	return S3StorageState{
		UUID:        uuid,
		Name:        inputs.Name,
		Description: inputs.Description,
		Endpoint:    inputs.Endpoint,
		Bucket:      inputs.Bucket,
		Region:      inputs.Region,
		IsUsable:    isUsable,
	}
}

func s3Placeholder(inputs S3StorageArgs) S3StorageState {
	state := s3State("pending", inputs, true)
	return state
}
