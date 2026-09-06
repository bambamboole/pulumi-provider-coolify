package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

func addApp(fake *fakeCoolify) string {
	projectUUID := fake.addProject("Main", "production")
	return fake.addApplication(map[string]any{"name": "app", "environment_id": fake.environmentID(projectUUID, "production"), "settings": map[string]any{}})
}

func TestCreateStorageCreatesAdoptsByMountPathAndPatches(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	appUUID := addApp(fake)

	args := StorageArgs{ApplicationUUID: appUUID, Type: StorageTypePersistent, MountPath: "/data", Name: "data"}
	storage, err := createStorage(ctx, c, args)
	if err != nil {
		t.Fatalf("createStorage: %v", err)
	}
	if storage.Type() != coolify.StoragePersistent || storage.Name != appUUID+"-data" || storage.MountPath != "/data" {
		t.Fatalf("unexpected storage: %+v", storage)
	}
	state := storageState(args, storage)
	if state.VolumeName != appUUID+"-data" || state.UUID != storage.UUID {
		t.Fatalf("state not mapped: %+v", state)
	}

	// Same mount path: adopt; the prefixed name equals the declared one, so no patch.
	adopted, err := createStorage(ctx, c, args)
	if err != nil {
		t.Fatalf("second createStorage: %v", err)
	}
	if adopted.UUID != storage.UUID || fake.countRequests("POST", "/api/v1/applications/"+appUUID+"/storages") != 1 {
		t.Fatalf("storage was recreated: %v", fake.requests)
	}
	if fake.countRequests("PATCH", "/api/v1/applications/"+appUUID+"/storages") != 0 {
		t.Fatalf("unchanged storage must not be patched: %v", fake.requests)
	}

	// Rename and toggle the preview suffix: one patch.
	args.Name = "data-v2"
	args.IsPreviewSuffixEnabled = true
	patched, err := createStorage(ctx, c, args)
	if err != nil {
		t.Fatalf("createStorage with changes: %v", err)
	}
	if patched.Name != "data-v2" || !patched.IsPreviewSuffixEnabled || fake.countRequests("PATCH", "/api/v1/applications/"+appUUID+"/storages") != 1 {
		t.Fatalf("storage not patched: %+v %v", patched, fake.requests)
	}

	// Directory storage on the same owner.
	dir, err := createStorage(ctx, c, StorageArgs{ApplicationUUID: appUUID, Type: StorageTypeDirectory, MountPath: "/uploads", FsPath: "/srv/uploads"})
	if err != nil {
		t.Fatalf("createStorage directory: %v", err)
	}
	if dir.Type() != coolify.StorageDirectory || coolify.Deref(dir.FsPath) != "/srv/uploads" {
		t.Fatalf("unexpected directory storage: %+v", dir)
	}

	// Type mismatch on adoption is rejected.
	_, err = createStorage(ctx, c, StorageArgs{ApplicationUUID: appUUID, Type: StorageTypePersistent, MountPath: "/uploads", Name: "uploads"})
	if err == nil || !strings.Contains(err.Error(), `with type "directory", expected "persistent"`) {
		t.Fatalf("expected type mismatch error, got %v", err)
	}

	if err := c.DeleteStorage(ctx, coolify.Owner{Kind: coolify.OwnerApplication, UUID: appUUID}, dir.UUID); err != nil {
		t.Fatalf("DeleteStorage: %v", err)
	}
	if _, err := c.GetStorage(ctx, coolify.Owner{Kind: coolify.OwnerApplication, UUID: appUUID}, dir.UUID); !coolify.IsNotFound(err) {
		t.Fatalf("storage not deleted: %v", err)
	}
}

func TestStorageInputsStripOwnerPrefix(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	appUUID := addApp(fake)
	owner := coolify.Owner{Kind: coolify.OwnerApplication, UUID: appUUID}
	uuid := fake.addStorage(appUUID, true, map[string]any{"name": appUUID + "-cache", "mount_path": "/cache", "host_path": "/mnt/cache", "is_preview_suffix_enabled": true})
	storage, _ := c.GetStorage(context.Background(), owner, uuid)

	inputs := storageInputs(StorageArgs{ApplicationUUID: appUUID, Type: StorageTypePersistent, MountPath: "/old", Name: "old"}, owner, storage)
	if inputs.Name != "cache" || inputs.MountPath != "/cache" || !inputs.IsPreviewSuffixEnabled {
		t.Fatalf("inputs must follow Coolify without the owner prefix: %+v", inputs)
	}
	if inputs.HostPath != "" {
		t.Fatalf("unset hostPath must stay unmanaged, got %q", inputs.HostPath)
	}
}

func TestDeleteStorageWithScheduleIsRejected(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	appUUID := addApp(fake)
	owner := coolify.Owner{Kind: coolify.OwnerApplication, UUID: appUUID}
	uuid := fake.addStorage(appUUID, true, map[string]any{"name": "data", "mount_path": "/data"})
	if _, err := c.SetVolumeBackup(ctx, owner, uuid, volumeBackupBody(VolumeBackupArgs{Frequency: "daily", Enabled: true})); err != nil {
		t.Fatalf("SetVolumeBackup: %v", err)
	}
	if err := c.DeleteStorage(ctx, owner, uuid); err == nil || !strings.Contains(err.Error(), "backup schedule") {
		t.Fatalf("expected Coolify's 422, got %v", err)
	}
}
