package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

func backupArgs(databaseUUID string) DatabaseBackupArgs {
	return DatabaseBackupArgs{DatabaseUUID: databaseUUID, Frequency: "0 3 * * *", Enabled: true}
}

func addPostgres(fake *fakeCoolify) string {
	projectUUID := fake.addProject("Main", "production")
	return fake.addDatabase(map[string]any{
		"name": "app-db", "database_type": "standalone-postgresql", "image": "postgres:16",
		"environment_id": fake.environmentID(projectUUID, "production"),
	})
}

func TestCreateDatabaseBackupCreatesAdoptsByFrequencyAndPatches(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	dbUUID := addPostgres(fake)
	fake.addBackup(dbUUID, map[string]any{"frequency": "0 0 * * *", "enabled": true, "save_s3": false})

	seven := 7
	args := backupArgs(dbUUID)
	args.RetentionAmountLocally = &seven
	backup, err := createDatabaseBackup(ctx, c, args)
	if err != nil {
		t.Fatalf("createDatabaseBackup: %v", err)
	}
	if backup.Frequency != "0 3 * * *" || !backup.Enabled || backup.RetentionAmountLocally == nil || *backup.RetentionAmountLocally != 7 {
		t.Fatalf("unexpected backup: %+v", backup)
	}
	if backups, _ := c.ListDatabaseBackups(ctx, dbUUID); len(backups) != 2 {
		t.Fatalf("a different frequency must create a second configuration: %+v", backups)
	}

	// Same frequency: adopt and patch instead of creating a third one.
	zero := 0
	args.Enabled = false
	args.RetentionAmountLocally = &zero
	adopted, err := createDatabaseBackup(ctx, c, args)
	if err != nil {
		t.Fatalf("second createDatabaseBackup: %v", err)
	}
	if adopted.UUID != backup.UUID || adopted.Enabled || *adopted.RetentionAmountLocally != 0 {
		t.Fatalf("backup was not adopted and patched: %+v", adopted)
	}
	if fake.countRequests("POST", "/api/v1/databases/"+dbUUID+"/backups") != 1 {
		t.Fatalf("backup was recreated: %v", fake.requests)
	}

	// Adopting Coolify's default schedule by its exact frequency string.
	defaults := backupArgs(dbUUID)
	defaults.Frequency = "0 0 * * *"
	if adopted, err := createDatabaseBackup(ctx, c, defaults); err != nil || adopted.Frequency != "0 0 * * *" || adopted.UUID == backup.UUID {
		t.Fatalf("default schedule not adopted: %+v %v", adopted, err)
	}

	if _, err := c.GetDatabaseBackup(ctx, dbUUID, "missing"); !coolify.IsNotFound(err) {
		t.Fatalf("missing backup must be not found, got %v", err)
	}
	if err := c.DeleteDatabaseBackup(ctx, dbUUID, backup.UUID, true); err != nil {
		t.Fatalf("DeleteDatabaseBackup: %v", err)
	}
	if !strings.HasSuffix(fake.lastRequest(), "?delete_s3=true") {
		t.Fatalf("delete must forward delete_s3: %s", fake.lastRequest())
	}
	if _, err := c.GetDatabaseBackup(ctx, dbUUID, backup.UUID); !coolify.IsNotFound(err) {
		t.Fatalf("backup not deleted: %v", err)
	}
}

func TestApplyDatabaseBackupSendsS3StorageWhenUnknownOrChanged(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	dbUUID := addPostgres(fake)
	uuid := fake.addBackup(dbUUID, map[string]any{"frequency": "0 3 * * *", "enabled": true, "save_s3": true, "s3_storage_id": 1})
	current, _ := c.GetDatabaseBackup(ctx, dbUUID, uuid)

	args := backupArgs(dbUUID)
	args.SaveS3 = true
	args.S3StorageUUID = "u-s3-new"

	// Unknown previous storage (adoption): the storage is sent even though
	// nothing else differs.
	if _, err := applyDatabaseBackup(ctx, c, current, args, nil); err != nil {
		t.Fatalf("applyDatabaseBackup: %v", err)
	}
	if fake.countRequests("PATCH", "/api/v1/databases/"+dbUUID+"/backups/") != 1 {
		t.Fatalf("expected a patch for the unknown storage: %v", fake.requests)
	}

	// Same storage as before: no patch.
	previous := "u-s3-new"
	if _, err := applyDatabaseBackup(ctx, c, current, args, &previous); err != nil {
		t.Fatalf("idempotent applyDatabaseBackup: %v", err)
	}
	if fake.countRequests("PATCH", "/api/v1/databases/"+dbUUID+"/backups/") != 1 {
		t.Fatalf("unchanged storage must not patch: %v", fake.requests)
	}

	// Storage changed: patch again.
	args.S3StorageUUID = "u-s3-other"
	if _, err := applyDatabaseBackup(ctx, c, current, args, &previous); err != nil {
		t.Fatalf("applyDatabaseBackup with changed storage: %v", err)
	}
	if fake.countRequests("PATCH", "/api/v1/databases/"+dbUUID+"/backups/") != 2 {
		t.Fatalf("changed storage must patch: %v", fake.requests)
	}

	// Switching S3 on always carries the storage, as Coolify requires.
	off := fake.addBackup(dbUUID, map[string]any{"frequency": "daily", "enabled": true, "save_s3": false})
	current, _ = c.GetDatabaseBackup(ctx, dbUUID, off)
	args.Frequency = "daily"
	updated, err := applyDatabaseBackup(ctx, c, current, args, &args.S3StorageUUID)
	if err != nil {
		t.Fatalf("enabling save_s3: %v", err)
	}
	if !updated.SaveS3 || updated.S3StorageID == nil {
		t.Fatalf("save_s3 must be enabled with a storage: %+v", updated)
	}
}

func TestCreateDatabaseBackupPropagatesUnsupportedEngine(t *testing.T) {
	fake := newFakeCoolify(t)
	projectUUID := fake.addProject("Main", "production")
	redis := fake.addDatabase(map[string]any{
		"name": "cache", "database_type": "standalone-redis", "image": "redis:7",
		"environment_id": fake.environmentID(projectUUID, "production"),
	})
	_, err := createDatabaseBackup(context.Background(), fake.client(), backupArgs(redis))
	if err == nil || !strings.Contains(err.Error(), "not supported for this database type") {
		t.Fatalf("expected Coolify's 422 to surface, got %v", err)
	}
}

func TestDatabaseBackupInputsKeepUnmanagedAndUnreported(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	dbUUID := addPostgres(fake)
	uuid := fake.addBackup(dbUUID, map[string]any{
		"frequency": "hourly", "enabled": false, "save_s3": true, "s3_storage_id": 3, "dump_all": true,
		"databases_to_backup": "app,analytics", "database_backup_retention_amount_locally": 3, "timeout": 600,
	})
	backup, _ := c.GetDatabaseBackup(context.Background(), dbUUID, uuid)

	previous := backupArgs(dbUUID)
	previous.SaveS3 = true
	previous.S3StorageUUID = "u-s3"
	inputs := databaseBackupInputs(previous, backup)
	if inputs.Frequency != "hourly" || inputs.Enabled || !inputs.SaveS3 || !inputs.DumpAll {
		t.Fatalf("managed fields must follow Coolify: %+v", inputs)
	}
	if inputs.S3StorageUUID != "u-s3" {
		t.Fatalf("the storage UUID is never reported and must be kept, got %q", inputs.S3StorageUUID)
	}
	if inputs.DatabasesToBackup != "" || inputs.RetentionAmountLocally != nil || inputs.Timeout != 0 {
		t.Fatalf("unset optional inputs must stay unmanaged: %+v", inputs)
	}
	one := 1
	previous.RetentionAmountLocally = &one
	previous.Timeout = 120
	inputs = databaseBackupInputs(previous, backup)
	if inputs.RetentionAmountLocally == nil || *inputs.RetentionAmountLocally != 3 || inputs.Timeout != 600 {
		t.Fatalf("managed optional inputs must reflect drift: %+v", inputs)
	}
}
