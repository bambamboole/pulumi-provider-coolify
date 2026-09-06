package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// addComposeService registers a service with two sub-containers that both
// mount /data plus one directory mount, like Coolify creates from a compose file.
func addComposeService(fake *fakeCoolify) string {
	projectUUID := fake.addProject("Main", "production")
	svc := fake.addService(map[string]any{"name": "gitea", "environment_id": fake.environmentID(projectUUID, "production")})
	fake.addStorage(svc, true, map[string]any{"name": "gitea-data", "mount_path": "/data", "resource_uuid": "u-sa-gitea", "resource_type": "application"})
	fake.addStorage(svc, true, map[string]any{"name": "mysql-data", "mount_path": "/data", "resource_uuid": "u-sd-mysql", "resource_type": "database"})
	fake.addStorage(svc, false, map[string]any{"mount_path": "/etc/gitea", "fs_path": "/srv/gitea/etc", "is_directory": true, "is_host_file": false, "resource_uuid": "u-sa-gitea", "resource_type": "application"})
	fake.addStorage(svc, false, map[string]any{"mount_path": "/etc/gitea/app.ini", "fs_path": "/srv/gitea/app.ini", "is_directory": false, "is_host_file": false, "resource_uuid": "u-sa-gitea", "resource_type": "application"})
	return svc
}

func TestFindStorageMatchesByMountPathAndName(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	svc := addComposeService(fake)
	owner := coolify.StorageOwner{Kind: coolify.OwnerService, UUID: svc}

	_, err := findStorage(ctx, c, owner, "/data", "")
	if err == nil || !strings.Contains(err.Error(), "2 storages match") || !strings.Contains(err.Error(), "gitea-data, mysql-data") {
		t.Fatalf("ambiguous mount path must list the names, got %v", err)
	}
	storage, err := findStorage(ctx, c, owner, "/data", "mysql-data")
	if err != nil || storage.ResourceUUID != "u-sd-mysql" || storage.Type() != coolify.StoragePersistent {
		t.Fatalf("expected the mysql volume, got %+v %v", storage, err)
	}
	dir, err := findStorage(ctx, c, owner, "/etc/gitea", "")
	if err != nil || dir.Type() != coolify.StorageDirectory {
		t.Fatalf("expected the directory mount, got %+v %v", dir, err)
	}
	file, err := findStorage(ctx, c, owner, "/etc/gitea/app.ini", "")
	if err != nil || file.Type() != coolify.StorageFile {
		t.Fatalf("expected the file mount, got %+v %v", file, err)
	}
	_, err = findStorage(ctx, c, owner, "/missing", "")
	if err == nil || !strings.Contains(err.Error(), "available: /data (gitea-data)") {
		t.Fatalf("missing mount path must list the available ones, got %v", err)
	}
	if _, err := findStorage(ctx, c, owner, "", ""); err == nil {
		t.Fatal("a filter is required")
	}

	// Names created through the API carry the owner prefix; both spellings match.
	appUUID := addApp(fake)
	fake.addStorage(appUUID, true, map[string]any{"name": appUUID + "-data", "mount_path": "/data"})
	appOwner := coolify.StorageOwner{Kind: coolify.OwnerApplication, UUID: appUUID}
	for _, name := range []string{"data", appUUID + "-data"} {
		if _, err := findStorage(ctx, c, appOwner, "", name); err != nil {
			t.Fatalf("name %q must match: %v", name, err)
		}
	}

	result, err := GetStorage{}.Invoke(withClient(ctx, c), infer.FunctionRequest[GetStorageArgs]{Input: GetStorageArgs{ServiceUUID: svc, MountPath: "/data", Name: "gitea-data"}})
	if err != nil || result.Output.ResourceUUID != "u-sa-gitea" || result.Output.Type != "persistent" {
		t.Fatalf("getStorage: %+v %v", result.Output, err)
	}
	if _, err := storageOwner("a", "", "s"); err == nil {
		t.Fatal("two owners must be rejected")
	}
}

func TestVolumeBackupResolvesMountPathAndSendsFullBody(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	svc := addComposeService(fake)
	owner := coolify.StorageOwner{Kind: coolify.OwnerService, UUID: svc}

	args := VolumeBackupArgs{
		ServiceUUID: svc, MountPath: "/data", VolumeName: "mysql-data",
		Frequency: "0 3 * * *", Enabled: true, SaveS3: true, S3StorageUUID: "u-s3", StopDuringBackup: true,
		RetentionAmountLocally: 7, RetentionAmountS3: 30, RetentionMaxStorageS3: 2.5,
	}
	resolvedOwner, storageUUID, err := resolveVolumeBackupTarget(ctx, c, args)
	if err != nil || resolvedOwner != owner {
		t.Fatalf("resolveVolumeBackupTarget: %s %v", storageUUID, err)
	}
	backup, err := c.SetVolumeBackup(ctx, owner, storageUUID, volumeBackupBody(args))
	if err != nil {
		t.Fatalf("SetVolumeBackup: %v", err)
	}
	if backup.StorageUuid != storageUUID || backup.StorageType != "persistent" || !backup.StopDuringBackup || !backup.SaveS3 {
		t.Fatalf("unexpected schedule: %+v", backup)
	}
	if backup.RetentionAmountS3 != 30 || backup.RetentionMaxStorageS3 != 2.5 || backup.RetentionDaysLocally != 0 || backup.Timeout != 3600 {
		t.Fatalf("body must carry every retention field and leave the timeout alone: %+v", backup)
	}
	state := volumeBackupState(args, backup)
	if state.UUID != backup.Uuid || state.ResolvedStorageUUID != storageUUID || state.StorageType != "persistent" {
		t.Fatalf("state not mapped: %+v", state)
	}

	// The upsert is idempotent: same storage, same schedule uuid.
	again, err := c.SetVolumeBackup(ctx, owner, storageUUID, volumeBackupBody(args))
	if err != nil || again.Uuid != backup.Uuid {
		t.Fatalf("second upsert must keep the schedule: %+v %v", again, err)
	}

	// Explicit storage UUID skips the lookup.
	direct := VolumeBackupArgs{ServiceUUID: svc, StorageUUID: storageUUID, Frequency: "daily"}
	lists := fake.countRequests("GET", "/api/v1/services/"+svc+"/storages")
	if _, got, err := resolveVolumeBackupTarget(ctx, c, direct); err != nil || got != storageUUID {
		t.Fatalf("explicit storage uuid: %s %v", got, err)
	}
	if fake.countRequests("GET", "/api/v1/services/"+svc+"/storages") != lists {
		t.Fatalf("explicit storage uuid must not list storages: %v", fake.requests)
	}

	// Single file mounts cannot be backed up.
	_, fileUUID, _ := resolveVolumeBackupTarget(ctx, c, VolumeBackupArgs{ServiceUUID: svc, MountPath: "/etc/gitea/app.ini", Frequency: "daily"})
	if _, err := c.SetVolumeBackup(ctx, owner, fileUUID, volumeBackupBody(direct)); err == nil || !strings.Contains(err.Error(), "Only directory file storages") {
		t.Fatalf("expected Coolify's 422 for file mounts, got %v", err)
	}

	if err := c.RunVolumeBackup(ctx, owner, storageUUID); err != nil || fake.volumeRuns[storageUUID] != 1 {
		t.Fatalf("RunVolumeBackup: %v runs=%d", err, fake.volumeRuns[storageUUID])
	}
	if err := c.DeleteVolumeBackup(ctx, owner, storageUUID); err != nil {
		t.Fatalf("DeleteVolumeBackup: %v", err)
	}
	if err := c.DeleteVolumeBackup(ctx, owner, storageUUID); !coolify.IsNotFound(err) {
		t.Fatalf("second delete must be not found, got %v", err)
	}
}

func TestVolumeBackupLifecycleThroughResource(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := withClient(context.Background(), c)
	appUUID := addApp(fake)
	storageUUID := fake.addStorage(appUUID, true, map[string]any{"name": appUUID + "-data", "mount_path": "/data"})

	args := VolumeBackupArgs{ApplicationUUID: appUUID, MountPath: "/data", Frequency: "daily", Enabled: true, RetentionAmountLocally: 7, RetentionAmountS3: 7, BackupNow: true}
	created, err := VolumeBackup{}.Create(ctx, infer.CreateRequest[VolumeBackupArgs]{Name: "data", Inputs: args})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Output.ResolvedStorageUUID != storageUUID || fake.volumeRuns[storageUUID] != 1 {
		t.Fatalf("create must resolve the storage and run once: %+v runs=%d", created.Output, fake.volumeRuns[storageUUID])
	}

	// Read keeps the inputs while the storage exists.
	read, err := VolumeBackup{}.Read(ctx, infer.ReadRequest[VolumeBackupArgs, VolumeBackupState]{ID: created.ID, Inputs: args, State: created.Output})
	if err != nil || read.ID != created.ID || read.Inputs.Frequency != "daily" {
		t.Fatalf("Read: %+v %v", read, err)
	}

	// Update re-sends the whole schedule to the resolved storage.
	args.Frequency = "0 4 * * *"
	args.BackupNow = false
	updated, err := VolumeBackup{}.Update(ctx, infer.UpdateRequest[VolumeBackupArgs, VolumeBackupState]{ID: created.ID, Inputs: args, State: created.Output})
	if err != nil || updated.Output.UUID != created.ID || fake.volumeBackups[storageUUID]["frequency"] != "0 4 * * *" {
		t.Fatalf("Update: %+v %v", updated.Output, err)
	}
	if fake.volumeRuns[storageUUID] != 1 {
		t.Fatal("update must not run a backup")
	}

	// Storage gone: Read reports the resource as deleted.
	delete(fake.volumeBackups, storageUUID)
	fake.storages[appUUID] = nil
	gone, err := VolumeBackup{}.Read(ctx, infer.ReadRequest[VolumeBackupArgs, VolumeBackupState]{ID: created.ID, Inputs: args, State: updated.Output})
	if err != nil || gone.ID != "" {
		t.Fatalf("Read of a vanished storage must return empty, got %+v %v", gone, err)
	}

	// Delete tolerates a missing schedule.
	_, err = VolumeBackup{}.Delete(ctx, infer.DeleteRequest[VolumeBackupState]{ID: created.ID, State: updated.Output})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
