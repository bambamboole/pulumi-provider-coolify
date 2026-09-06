package coolify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// OwnerKind is the kind of resource a storage or volume backup belongs to.
type OwnerKind string

const (
	OwnerApplication OwnerKind = "application"
	OwnerDatabase    OwnerKind = "database"
	OwnerService     OwnerKind = "service"
)

// StorageOwner addresses the resource whose storages are managed.
type StorageOwner struct {
	Kind OwnerKind
	UUID string
}

// StorageType distinguishes the storage rows Coolify returns.
type StorageType string

const (
	// StoragePersistent is a named Docker volume.
	StoragePersistent StorageType = "persistent"
	// StorageDirectory is a host directory mounted into the container.
	StorageDirectory StorageType = "directory"
	// StorageFile is a single file mount; it cannot be backed up.
	StorageFile StorageType = "file"
)

// Storage is a persistent volume or a file/directory mount. The storage
// endpoints are untyped in the OpenAPI specification, so the model is
// hand-written from the JSON Coolify actually returns. For service storages
// ResourceUUID names the compose sub-resource the storage belongs to.
type Storage struct {
	UUID                   string  `json:"uuid"`
	Name                   string  `json:"name"`
	MountPath              string  `json:"mount_path"`
	HostPath               *string `json:"host_path"`
	FsPath                 *string `json:"fs_path"`
	IsDirectory            bool    `json:"is_directory"`
	IsHostFile             bool    `json:"is_host_file"`
	IsPreviewSuffixEnabled bool    `json:"is_preview_suffix_enabled"`
	ResourceUUID           string  `json:"resource_uuid"`
	ResourceType           string  `json:"resource_type"`

	persistent bool
}

// Type reports the storage type; persistent rows come from the
// persistent_storages list, the others from file_storages.
func (s Storage) Type() StorageType {
	switch {
	case s.persistent:
		return StoragePersistent
	case s.IsDirectory && !s.IsHostFile:
		return StorageDirectory
	default:
		return StorageFile
	}
}

type storageList struct {
	PersistentStorages []Storage `json:"persistent_storages"`
	FileStorages       []Storage `json:"file_storages"`
}

// ListStorages returns the persistent volumes followed by the file and
// directory mounts of the owner.
func (c *Client) ListStorages(ctx context.Context, owner StorageOwner) ([]Storage, error) {
	var resp *http.Response
	var err error
	switch owner.Kind {
	case OwnerApplication:
		resp, err = c.api.ListStoragesByApplicationUuid(ctx, owner.UUID)
	case OwnerDatabase:
		resp, err = c.api.ListStoragesByDatabaseUuid(ctx, owner.UUID)
	case OwnerService:
		resp, err = c.api.ListStoragesByServiceUuid(ctx, owner.UUID)
	default:
		return nil, fmt.Errorf("coolify: unsupported storage owner %q", owner.Kind)
	}
	list, err := decode[storageList](resp, err)
	if err != nil {
		return nil, err
	}
	out := make([]Storage, 0, len(list.PersistentStorages)+len(list.FileStorages))
	for _, storage := range list.PersistentStorages {
		storage.persistent = true
		out = append(out, storage)
	}
	return append(out, list.FileStorages...), nil
}

// GetStorage finds a storage of the owner by UUID. The API has no single
// storage endpoint, so it lists and returns a 404 APIError when missing.
func (c *Client) GetStorage(ctx context.Context, owner StorageOwner, storageUUID string) (Storage, error) {
	storages, err := c.ListStorages(ctx, owner)
	if err != nil {
		return Storage{}, err
	}
	for _, storage := range storages {
		if storage.UUID == storageUUID {
			return storage, nil
		}
	}
	return Storage{}, &APIError{
		Status: http.StatusNotFound,
		Method: http.MethodGet,
		Path:   storagePath(owner, storageUUID),
		Body:   `{"message":"Storage not found."}`,
	}
}

func storagePath(owner StorageOwner, storageUUID string) string {
	return apiPath + "/" + string(owner.Kind) + "s/" + owner.UUID + "/storages/" + storageUUID
}

// CreateStorageInput is the create body shared by applications and databases.
type CreateStorageInput struct {
	Type        StorageType `json:"type"`
	MountPath   string      `json:"mount_path"`
	Name        *string     `json:"name,omitempty"`
	HostPath    *string     `json:"host_path,omitempty"`
	IsDirectory *bool       `json:"is_directory,omitempty"`
	FsPath      *string     `json:"fs_path,omitempty"`
}

// UpdateStorageInput is the patch body shared by applications and databases.
// Type is required by Coolify and must be "persistent" or "file".
type UpdateStorageInput struct {
	UUID                   string  `json:"uuid"`
	Type                   string  `json:"type"`
	Name                   *string `json:"name,omitempty"`
	MountPath              *string `json:"mount_path,omitempty"`
	HostPath               *string `json:"host_path,omitempty"`
	IsPreviewSuffixEnabled *bool   `json:"is_preview_suffix_enabled,omitempty"`
}

// CreateStorage creates a storage on an application or database and returns
// its UUID. Directory mounts are created as file storages with is_directory.
func (c *Client) CreateStorage(ctx context.Context, owner StorageOwner, in CreateStorageInput) (string, error) {
	if in.Type == StorageDirectory {
		in.Type = StorageFile
		in.IsDirectory = Ptr(true)
	}
	var create uuidBodyRequest
	switch owner.Kind {
	case OwnerApplication:
		create = c.api.CreateStorageByApplicationUuidWithBody
	case OwnerDatabase:
		create = c.api.CreateStorageByDatabaseUuidWithBody
	default:
		return "", fmt.Errorf("coolify: storages cannot be created on a %s", owner.Kind)
	}
	return decodeUUID(postJSON(ctx, create, owner.UUID, in))
}

func (c *Client) UpdateStorage(ctx context.Context, owner StorageOwner, in UpdateStorageInput) error {
	var update uuidBodyRequest
	switch owner.Kind {
	case OwnerApplication:
		update = c.api.UpdateStorageByApplicationUuidWithBody
	case OwnerDatabase:
		update = c.api.UpdateStorageByDatabaseUuidWithBody
	default:
		return fmt.Errorf("coolify: storages cannot be updated on a %s", owner.Kind)
	}
	return check(postJSON(ctx, update, owner.UUID, in))
}

func (c *Client) DeleteStorage(ctx context.Context, owner StorageOwner, storageUUID string) error {
	switch owner.Kind {
	case OwnerApplication:
		return check(c.api.DeleteStorageByApplicationUuid(ctx, owner.UUID, storageUUID))
	case OwnerDatabase:
		return check(c.api.DeleteStorageByDatabaseUuid(ctx, owner.UUID, storageUUID))
	default:
		return fmt.Errorf("coolify: storages cannot be deleted on a %s", owner.Kind)
	}
}

// uuidBodyRequest is the shape of the generated *WithBody methods that address
// a parent resource by UUID.
type uuidBodyRequest func(context.Context, string, string, io.Reader, ...api.RequestEditorFn) (*http.Response, error)

// postJSON encodes in and sends it through one of the generated *WithBody methods.
func postJSON(ctx context.Context, send uuidBodyRequest, uuid string, in any) (*http.Response, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("coolify: encode request: %w", err)
	}
	return send(ctx, uuid, "application/json", bytes.NewReader(body))
}
