package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// GetStorage looks up a persistent volume or file/directory mount of an
// application, database or service.
type GetStorage struct{}

type GetStorageArgs struct {
	// UUID of the application owning the storage.
	ApplicationUUID string `pulumi:"applicationUuid,optional"`
	// UUID of the database owning the storage.
	DatabaseUUID string `pulumi:"databaseUuid,optional"`
	// UUID of the service owning the storage.
	ServiceUUID string `pulumi:"serviceUuid,optional"`
	// Container mount path to look for.
	MountPath string `pulumi:"mountPath,optional"`
	// Volume name to look for, with or without Coolify's owner prefix.
	Name string `pulumi:"name,optional"`
}

type GetStorageResult struct {
	// UUID of the storage.
	UUID string `pulumi:"uuid"`
	// Storage type: persistent, directory or file.
	Type string `pulumi:"type"`
	// Volume name as reported by Coolify.
	Name string `pulumi:"name"`
	// Container mount path.
	MountPath string `pulumi:"mountPath"`
	// Host path of a persistent volume, when bound to one.
	HostPath string `pulumi:"hostPath"`
	// Host directory of a directory mount.
	FsPath string `pulumi:"fsPath"`
	// UUID of the compose sub-resource the storage belongs to, for services.
	ResourceUUID string `pulumi:"resourceUuid"`
}

func (r *GetStorage) Annotate(a infer.Annotator) {
	a.SetToken("index", "getStorage")
	a.Describe(&r, "Looks up a storage (persistent volume or file/directory mount) of an application, database or service by mount path and/or volume name, e.g. to back it up with a VolumeBackup.")
}

func (args *GetStorageArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ApplicationUUID, "UUID of the application owning the storage. Exactly one of applicationUuid, databaseUuid and serviceUuid must be set.")
	a.Describe(&args.DatabaseUUID, "UUID of the database owning the storage. Exactly one of applicationUuid, databaseUuid and serviceUuid must be set.")
	a.Describe(&args.ServiceUUID, "UUID of the service owning the storage. Exactly one of applicationUuid, databaseUuid and serviceUuid must be set.")
	a.Describe(&args.MountPath, "Container mount path to look for. At least one of mountPath and name must be set.")
	a.Describe(&args.Name, "Volume name to look for, with or without Coolify's owner prefix. At least one of mountPath and name must be set.")
}

func (result *GetStorageResult) Annotate(a infer.Annotator) {
	a.Describe(&result.UUID, "UUID of the storage.")
	a.Describe(&result.Type, "Storage type: persistent, directory or file.")
	a.Describe(&result.Name, "Volume name as reported by Coolify.")
	a.Describe(&result.MountPath, "Container mount path.")
	a.Describe(&result.HostPath, "Host path of a persistent volume, when bound to one.")
	a.Describe(&result.FsPath, "Host directory of a directory mount.")
	a.Describe(&result.ResourceUUID, "UUID of the compose sub-resource the storage belongs to, for services.")
}

func (GetStorage) Invoke(ctx context.Context, req infer.FunctionRequest[GetStorageArgs]) (infer.FunctionResponse[GetStorageResult], error) {
	args := req.Input
	owner, err := storageOwner(args.ApplicationUUID, args.DatabaseUUID, args.ServiceUUID)
	if err != nil {
		return infer.FunctionResponse[GetStorageResult]{}, err
	}
	storage, err := findStorage(ctx, client(ctx), owner, args.MountPath, args.Name)
	if err != nil {
		return infer.FunctionResponse[GetStorageResult]{}, err
	}
	return infer.FunctionResponse[GetStorageResult]{Output: GetStorageResult{
		UUID:         storage.UUID,
		Type:         string(storage.Type()),
		Name:         storage.Name,
		MountPath:    storage.MountPath,
		HostPath:     coolify.Deref(storage.HostPath),
		FsPath:       coolify.Deref(storage.FsPath),
		ResourceUUID: storage.ResourceUUID,
	}}, nil
}

// storageOwner derives the owner from the three mutually exclusive UUID inputs.
func storageOwner(applicationUUID, databaseUUID, serviceUUID string) (coolify.Owner, error) {
	var owner coolify.Owner
	set := 0
	for kind, uuid := range map[coolify.OwnerKind]string{
		coolify.OwnerApplication: applicationUUID,
		coolify.OwnerDatabase:    databaseUUID,
		coolify.OwnerService:     serviceUUID,
	} {
		if uuid != "" {
			owner = coolify.Owner{Kind: kind, UUID: uuid}
			set++
		}
	}
	if set != 1 {
		return coolify.Owner{}, fmt.Errorf("exactly one of applicationUuid, databaseUuid and serviceUuid must be set")
	}
	return owner, nil
}

// findStorage returns the single storage of the owner matching the mount path
// and/or name. Names match the value Coolify reports or the value without the
// owner prefix Coolify adds to volumes created through the API.
func findStorage(ctx context.Context, c *coolify.Client, owner coolify.Owner, mountPath, name string) (coolify.Storage, error) {
	if mountPath == "" && name == "" {
		return coolify.Storage{}, fmt.Errorf("at least one of mountPath and name must be set")
	}
	storages, err := c.ListStorages(ctx, owner)
	if err != nil {
		return coolify.Storage{}, err
	}
	var matches []coolify.Storage
	for _, storage := range storages {
		if mountPath != "" && storage.MountPath != mountPath {
			continue
		}
		if name != "" && storage.Name != name && !strings.HasSuffix(storage.Name, "-"+name) {
			continue
		}
		matches = append(matches, storage)
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		available := make([]string, 0, len(storages))
		for _, storage := range storages {
			available = append(available, fmt.Sprintf("%s (%s)", storage.MountPath, storage.Name))
		}
		sort.Strings(available)
		return coolify.Storage{}, fmt.Errorf("no storage matching mountPath=%q name=%q on %s %s; available: %s",
			mountPath, name, owner.Kind, owner.UUID, strings.Join(available, ", "))
	default:
		names := make([]string, 0, len(matches))
		for _, storage := range matches {
			names = append(names, storage.Name)
		}
		sort.Strings(names)
		return coolify.Storage{}, fmt.Errorf("%d storages match mountPath=%q on %s %s; set name to one of: %s",
			len(matches), mountPath, owner.Kind, owner.UUID, strings.Join(names, ", "))
	}
}
