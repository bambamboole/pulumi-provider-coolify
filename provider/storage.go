package provider

import (
	"context"
	"fmt"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// StorageType selects the kind of storage to mount.
type StorageType string

const (
	StorageTypePersistent StorageType = StorageType(coolify.StoragePersistent)
	StorageTypeDirectory  StorageType = StorageType(coolify.StorageDirectory)
)

func (StorageType) Values() []infer.EnumValue[StorageType] {
	return []infer.EnumValue[StorageType]{
		{Name: "Persistent", Value: StorageTypePersistent, Description: "Named Docker volume."},
		{Name: "Directory", Value: StorageTypeDirectory, Description: "Host directory mounted into the container."},
	}
}

// Storage manages a persistent volume or directory mount of an application or database.
type Storage struct{}

type StorageArgs struct {
	// UUID of the application the storage is mounted into.
	ApplicationUUID string `pulumi:"applicationUuid,optional"`
	// UUID of the database the storage is mounted into.
	DatabaseUUID string `pulumi:"databaseUuid,optional"`
	// Kind of storage.
	Type StorageType `pulumi:"type"`
	// Container mount path. An existing storage with this mount path on the owner is adopted.
	MountPath string `pulumi:"mountPath"`
	// Volume name for persistent storages, without the owner prefix Coolify adds.
	Name string `pulumi:"name,optional"`
	// Host path to bind a persistent volume to.
	HostPath string `pulumi:"hostPath,optional"`
	// Host directory for directory storages.
	FsPath string `pulumi:"fsPath,optional"`
	// Add a -pr-N suffix for preview deployments.
	IsPreviewSuffixEnabled bool `pulumi:"isPreviewSuffixEnabled,optional"`
}

type StorageState struct {
	StorageArgs
	// UUID of the storage in Coolify.
	UUID string `pulumi:"uuid"`
	// Full volume name as reported by Coolify, including the owner prefix.
	VolumeName string `pulumi:"volumeName"`
}

func (r *Storage) Annotate(a infer.Annotator) {
	a.SetToken("index", "Storage")
	a.Describe(&r, "A persistent volume or directory mount of a Coolify application or database. An existing storage with the same mount path on the owner is adopted on create. Volumes declared in a docker compose file are managed by Coolify itself and cannot be created here; look them up with getStorage instead.")
}

func (args *StorageArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ApplicationUUID, "UUID of the application the storage is mounted into. Exactly one of applicationUuid and databaseUuid must be set. Changing it replaces the storage.")
	a.Describe(&args.DatabaseUUID, "UUID of the database the storage is mounted into. Exactly one of applicationUuid and databaseUuid must be set. Changing it replaces the storage.")
	a.Describe(&args.Type, "Kind of storage. Changing it replaces the storage.")
	a.Describe(&args.MountPath, "Container mount path. An existing storage with this mount path on the owner is adopted on create.")
	a.Describe(&args.Name, "Volume name for persistent storages, without the owner prefix Coolify adds. Required for persistent storages.")
	a.Describe(&args.HostPath, "Host path to bind a persistent volume to. Leave unset for a plain Docker volume.")
	a.Describe(&args.FsPath, "Host directory for directory storages. Required for directory storages; changing it replaces the storage.")
	a.Describe(&args.IsPreviewSuffixEnabled, "Add a -pr-N suffix for preview deployments.")
}

func (state *StorageState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the storage in Coolify.")
	a.Describe(&state.VolumeName, "Full volume name as reported by Coolify, including the owner prefix.")
}

func (Storage) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[StorageArgs], error) {
	args, failures, err := infer.DefaultCheck[StorageArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[StorageArgs]{}, err
	}
	if (args.ApplicationUUID == "") == (args.DatabaseUUID == "") {
		failures = append(failures, p.CheckFailure{Property: "applicationUuid", Reason: "exactly one of applicationUuid and databaseUuid must be set"})
	}
	switch args.Type {
	case StorageTypePersistent:
		if args.Name == "" {
			failures = append(failures, p.CheckFailure{Property: "name", Reason: "name is required for persistent storages"})
		}
		if args.FsPath != "" {
			failures = append(failures, p.CheckFailure{Property: "fsPath", Reason: "fsPath is only valid for directory storages"})
		}
	case StorageTypeDirectory:
		if args.FsPath == "" {
			failures = append(failures, p.CheckFailure{Property: "fsPath", Reason: "fsPath is required for directory storages"})
		}
		if args.Name != "" || args.HostPath != "" {
			failures = append(failures, p.CheckFailure{Property: "name", Reason: "name and hostPath are only valid for persistent storages"})
		}
	}
	return infer.CheckResponse[StorageArgs]{Inputs: args, Failures: failures}, nil
}

func (Storage) Create(ctx context.Context, req infer.CreateRequest[StorageArgs]) (infer.CreateResponse[StorageState], error) {
	if req.DryRun {
		return infer.CreateResponse[StorageState]{Output: StorageState{StorageArgs: req.Inputs}}, nil
	}
	storage, err := createStorage(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.CreateResponse[StorageState]{}, err
	}
	return infer.CreateResponse[StorageState]{ID: storage.UUID, Output: storageState(req.Inputs, storage)}, nil
}

func (Storage) Diff(ctx context.Context, req infer.DiffRequest[StorageArgs, StorageState]) (infer.DiffResponse, error) {
	diff := diffArgs(req.State.StorageArgs, req.Inputs, "applicationUuid", "databaseUuid", "type", "fsPath")
	return diffResponse(diff, req.State.MountPath == req.Inputs.MountPath), nil
}

func (Storage) Update(ctx context.Context, req infer.UpdateRequest[StorageArgs, StorageState]) (infer.UpdateResponse[StorageState], error) {
	if req.DryRun {
		state := req.State
		state.StorageArgs = req.Inputs
		return infer.UpdateResponse[StorageState]{Output: state}, nil
	}
	c := client(ctx)
	owner := storageArgsOwner(req.Inputs)
	current, err := c.GetStorage(ctx, owner, req.ID)
	if err != nil {
		return infer.UpdateResponse[StorageState]{}, err
	}
	storage, err := applyStorage(ctx, c, owner, current, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[StorageState]{}, err
	}
	return infer.UpdateResponse[StorageState]{Output: storageState(req.Inputs, storage)}, nil
}

func (Storage) Read(ctx context.Context, req infer.ReadRequest[StorageArgs, StorageState]) (infer.ReadResponse[StorageArgs, StorageState], error) {
	previous := req.Inputs
	if previous.ApplicationUUID == "" && previous.DatabaseUUID == "" {
		previous = req.State.StorageArgs
	}
	owner := storageArgsOwner(previous)
	storage, err := client(ctx).GetStorage(ctx, owner, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[StorageArgs, StorageState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[StorageArgs, StorageState]{}, err
	}
	inputs := storageInputs(previous, owner, storage)
	return infer.ReadResponse[StorageArgs, StorageState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  storageState(inputs, storage),
	}, nil
}

func (Storage) Delete(ctx context.Context, req infer.DeleteRequest[StorageState]) (infer.DeleteResponse, error) {
	err := client(ctx).DeleteStorage(ctx, storageArgsOwner(req.State.StorageArgs), req.ID)
	if err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func storageArgsOwner(args StorageArgs) coolify.StorageOwner {
	if args.DatabaseUUID != "" {
		return coolify.StorageOwner{Kind: coolify.OwnerDatabase, UUID: args.DatabaseUUID}
	}
	return coolify.StorageOwner{Kind: coolify.OwnerApplication, UUID: args.ApplicationUUID}
}

// createStorage adopts the storage with the same mount path on the owner or
// creates it, and reconciles its settings with the inputs.
func createStorage(ctx context.Context, c *coolify.Client, inputs StorageArgs) (coolify.Storage, error) {
	owner := storageArgsOwner(inputs)
	storages, err := c.ListStorages(ctx, owner)
	if err != nil {
		return coolify.Storage{}, err
	}
	for _, candidate := range storages {
		if candidate.MountPath != inputs.MountPath {
			continue
		}
		if candidate.Type() != coolify.StorageType(inputs.Type) {
			return coolify.Storage{}, fmt.Errorf("coolify storage at %q already exists on %s %s with type %q, expected %q",
				inputs.MountPath, owner.Kind, owner.UUID, candidate.Type(), inputs.Type)
		}
		return applyStorage(ctx, c, owner, candidate, inputs)
	}
	uuid, err := c.CreateStorage(ctx, owner, coolify.CreateStorageInput{
		Type:      coolify.StorageType(inputs.Type),
		MountPath: inputs.MountPath,
		Name:      coolify.PtrIfNonZero(inputs.Name),
		HostPath:  coolify.PtrIfNonZero(inputs.HostPath),
		FsPath:    coolify.PtrIfNonZero(inputs.FsPath),
	})
	if err != nil {
		return coolify.Storage{}, err
	}
	created, err := c.GetStorage(ctx, owner, uuid)
	if err != nil {
		return coolify.Storage{}, err
	}
	// The create endpoint ignores the preview suffix flag.
	return applyStorage(ctx, c, owner, created, inputs)
}

// applyStorage patches the fields of current that differ from the inputs.
func applyStorage(ctx context.Context, c *coolify.Client, owner coolify.StorageOwner, current coolify.Storage, inputs StorageArgs) (coolify.Storage, error) {
	body := coolify.UpdateStorageInput{UUID: current.UUID, Type: "file"}
	if current.Type() == coolify.StoragePersistent {
		body.Type = "persistent"
	}
	var patch patch
	patch.str(&body.MountPath, inputs.MountPath, current.MountPath)
	patch.str(&body.Name, inputs.Name, volumeShortName(owner, current.Name))
	patch.str(&body.HostPath, inputs.HostPath, coolify.Deref(current.HostPath))
	patch.boolean(&body.IsPreviewSuffixEnabled, inputs.IsPreviewSuffixEnabled, current.IsPreviewSuffixEnabled)
	if !patch.changed {
		return current, nil
	}
	if err := c.UpdateStorage(ctx, owner, body); err != nil {
		return coolify.Storage{}, err
	}
	return c.GetStorage(ctx, owner, current.UUID)
}

// volumeShortName strips the owner prefix Coolify adds to volume names created
// through the API, so names compare against what the program declared.
func volumeShortName(owner coolify.StorageOwner, name string) string {
	return strings.TrimPrefix(name, owner.UUID+"-")
}

// storageInputs derives the inputs from the storage Coolify reports.
func storageInputs(previous StorageArgs, owner coolify.StorageOwner, storage coolify.Storage) StorageArgs {
	inputs := previous
	inputs.Type = StorageType(storage.Type())
	inputs.MountPath = storage.MountPath
	inputs.IsPreviewSuffixEnabled = storage.IsPreviewSuffixEnabled
	if storage.Type() == coolify.StoragePersistent {
		inputs.Name = volumeShortName(owner, storage.Name)
		inputs.HostPath = ifSet(previous.HostPath, coolify.Deref(storage.HostPath))
	} else {
		inputs.FsPath = coolify.Deref(storage.FsPath)
	}
	return inputs
}

func storageState(inputs StorageArgs, storage coolify.Storage) StorageState {
	return StorageState{StorageArgs: inputs, UUID: storage.UUID, VolumeName: storage.Name}
}
