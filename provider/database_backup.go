package provider

import (
	"context"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// DatabaseBackup manages a scheduled backup configuration of a standalone database.
type DatabaseBackup struct{}

type DatabaseBackupArgs struct {
	// UUID of the Coolify database to back up.
	DatabaseUUID string `pulumi:"databaseUuid"`
	// Cron expression or Coolify shorthand. An existing configuration with the
	// identical frequency string on the database is adopted.
	Frequency string `pulumi:"frequency"`
	// Whether the schedule is enabled.
	Enabled bool `pulumi:"enabled,optional"`
	// Upload backups to an S3 storage.
	SaveS3 bool `pulumi:"saveS3,optional"`
	// UUID of the S3 storage. Required when saveS3 is true.
	S3StorageUUID string `pulumi:"s3StorageUuid,optional"`
	// Comma separated list of databases to dump. Leave unset to keep Coolify's default.
	DatabasesToBackup string `pulumi:"databasesToBackup,optional"`
	// Dump all databases of the server instead of the listed ones.
	DumpAll bool `pulumi:"dumpAll,optional"`
	// Number of backups kept locally; 0 keeps all.
	RetentionAmountLocally *int `pulumi:"retentionAmountLocally,optional"`
	// Days local backups are kept; 0 keeps them forever.
	RetentionDaysLocally *int `pulumi:"retentionDaysLocally,optional"`
	// Maximum local storage in GB; 0 is unlimited.
	RetentionMaxStorageLocally *float64 `pulumi:"retentionMaxStorageLocally,optional"`
	// Number of backups kept in S3; 0 keeps all.
	RetentionAmountS3 *int `pulumi:"retentionAmountS3,optional"`
	// Days S3 backups are kept; 0 keeps them forever.
	RetentionDaysS3 *int `pulumi:"retentionDaysS3,optional"`
	// Maximum S3 storage in GB; 0 is unlimited.
	RetentionMaxStorageS3 *float64 `pulumi:"retentionMaxStorageS3,optional"`
	// Timeout of a backup run in seconds (60 to 36000). Leave unset to keep Coolify's default.
	Timeout int `pulumi:"timeout,optional"`
	// Run a backup right after creating the configuration.
	BackupNow bool `pulumi:"backupNow,optional"`
	// Also delete the uploaded S3 files when the configuration is destroyed.
	DeleteS3Files bool `pulumi:"deleteS3Files,optional"`
}

type DatabaseBackupState struct {
	DatabaseBackupArgs
	// UUID of the backup configuration in Coolify.
	UUID string `pulumi:"uuid"`
}

func (r *DatabaseBackup) Annotate(a infer.Annotator) {
	a.SetToken("index", "DatabaseBackup")
	a.Describe(&r, "A scheduled backup configuration of a standalone Coolify database, optionally uploaded to an S3 storage. An existing configuration with the identical frequency on the database is adopted on create. Redis, KeyDB and Dragonfly do not support backups.")
}

func (args *DatabaseBackupArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.DatabaseUUID, "UUID of the Coolify database to back up (the uuid output of a Database resource). Changing it replaces the configuration.")
	a.Describe(&args.Frequency, `Cron expression or Coolify shorthand such as "daily" or "0 3 * * *". An existing configuration with the identical frequency string on the database is adopted on create; Coolify's own default schedule uses "0 0 * * *".`)
	a.Describe(&args.Enabled, "Whether the schedule is enabled.")
	a.Describe(&args.SaveS3, "Upload backups to an S3 storage.")
	a.Describe(&args.S3StorageUUID, "UUID of the S3 storage (the uuid output of an S3Storage resource). Required when saveS3 is true. Coolify does not report the configured storage, so drift on this input is not detected.")
	a.Describe(&args.DatabasesToBackup, "Comma separated list of databases to dump. Leave unset to keep Coolify's default, the database's primary database.")
	a.Describe(&args.DumpAll, "Dump all databases of the server instead of the listed ones.")
	a.Describe(&args.RetentionAmountLocally, "Number of backups kept locally; 0 keeps all. Leave unset to keep Coolify's default.")
	a.Describe(&args.RetentionDaysLocally, "Days local backups are kept; 0 keeps them forever. Leave unset to keep Coolify's default.")
	a.Describe(&args.RetentionMaxStorageLocally, "Maximum local storage in GB; 0 is unlimited. Leave unset to keep Coolify's default.")
	a.Describe(&args.RetentionAmountS3, "Number of backups kept in S3; 0 keeps all. Leave unset to keep Coolify's default.")
	a.Describe(&args.RetentionDaysS3, "Days S3 backups are kept; 0 keeps them forever. Leave unset to keep Coolify's default.")
	a.Describe(&args.RetentionMaxStorageS3, "Maximum S3 storage in GB; 0 is unlimited. Leave unset to keep Coolify's default.")
	a.Describe(&args.Timeout, "Timeout of a backup run in seconds, between 60 and 36000. Leave unset to keep Coolify's default.")
	a.Describe(&args.BackupNow, "Run a backup right after creating the configuration. Only relevant on create.")
	a.Describe(&args.DeleteS3Files, "Also delete the uploaded S3 files when the configuration is destroyed.")
	a.SetDefault(&args.Enabled, true)
}

func (state *DatabaseBackupState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the backup configuration in Coolify.")
}

func (DatabaseBackup) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[DatabaseBackupArgs], error) {
	args, failures, err := infer.DefaultCheck[DatabaseBackupArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[DatabaseBackupArgs]{}, err
	}
	if args.SaveS3 && args.S3StorageUUID == "" && !req.NewInputs.Get("s3StorageUuid").IsComputed() {
		failures = append(failures, p.CheckFailure{Property: "s3StorageUuid", Reason: "s3StorageUuid is required when saveS3 is true"})
	}
	return infer.CheckResponse[DatabaseBackupArgs]{Inputs: args, Failures: failures}, nil
}

func (DatabaseBackup) Create(ctx context.Context, req infer.CreateRequest[DatabaseBackupArgs]) (infer.CreateResponse[DatabaseBackupState], error) {
	if req.DryRun {
		return infer.CreateResponse[DatabaseBackupState]{Output: DatabaseBackupState{DatabaseBackupArgs: req.Inputs}}, nil
	}
	backup, err := createDatabaseBackup(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.CreateResponse[DatabaseBackupState]{}, err
	}
	return infer.CreateResponse[DatabaseBackupState]{ID: backup.UUID, Output: databaseBackupState(req.Inputs, backup)}, nil
}

func (DatabaseBackup) Diff(ctx context.Context, req infer.DiffRequest[DatabaseBackupArgs, DatabaseBackupState]) (infer.DiffResponse, error) {
	diff := diffArgs(req.State.DatabaseBackupArgs, req.Inputs, "databaseUuid")
	// Only relevant on create.
	delete(diff, "backupNow")
	// A replacement always targets another database, so the replacement can
	// never adopt the configuration that is about to be deleted.
	return diffResponse(diff, false), nil
}

func (DatabaseBackup) Update(ctx context.Context, req infer.UpdateRequest[DatabaseBackupArgs, DatabaseBackupState]) (infer.UpdateResponse[DatabaseBackupState], error) {
	if req.DryRun {
		return infer.UpdateResponse[DatabaseBackupState]{Output: DatabaseBackupState{DatabaseBackupArgs: req.Inputs, UUID: req.ID}}, nil
	}
	c := client(ctx)
	current, err := c.GetDatabaseBackup(ctx, req.Inputs.DatabaseUUID, req.ID)
	if err != nil {
		return infer.UpdateResponse[DatabaseBackupState]{}, err
	}
	backup, err := applyDatabaseBackup(ctx, c, current, req.Inputs, &req.State.S3StorageUUID)
	if err != nil {
		return infer.UpdateResponse[DatabaseBackupState]{}, err
	}
	return infer.UpdateResponse[DatabaseBackupState]{Output: databaseBackupState(req.Inputs, backup)}, nil
}

func (DatabaseBackup) Read(ctx context.Context, req infer.ReadRequest[DatabaseBackupArgs, DatabaseBackupState]) (infer.ReadResponse[DatabaseBackupArgs, DatabaseBackupState], error) {
	databaseUUID := firstNonEmpty(req.Inputs.DatabaseUUID, req.State.DatabaseUUID)
	backup, err := client(ctx).GetDatabaseBackup(ctx, databaseUUID, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[DatabaseBackupArgs, DatabaseBackupState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[DatabaseBackupArgs, DatabaseBackupState]{}, err
	}
	inputs := databaseBackupInputs(req.Inputs, backup)
	inputs.DatabaseUUID = databaseUUID
	return infer.ReadResponse[DatabaseBackupArgs, DatabaseBackupState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  databaseBackupState(inputs, backup),
	}, nil
}

func (DatabaseBackup) Delete(ctx context.Context, req infer.DeleteRequest[DatabaseBackupState]) (infer.DeleteResponse, error) {
	err := client(ctx).DeleteDatabaseBackup(ctx, req.State.DatabaseUUID, req.ID, req.State.DeleteS3Files)
	if err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

// createDatabaseBackup adopts the configuration with the identical frequency on
// the database or creates it, and reconciles its settings with the inputs.
func createDatabaseBackup(ctx context.Context, c *coolify.Client, inputs DatabaseBackupArgs) (coolify.DatabaseBackup, error) {
	backups, err := c.ListDatabaseBackups(ctx, inputs.DatabaseUUID)
	if err != nil {
		return coolify.DatabaseBackup{}, err
	}
	for _, backup := range backups {
		if backup.Frequency == inputs.Frequency {
			return applyDatabaseBackup(ctx, c, backup, inputs, nil)
		}
	}
	uuid, err := c.CreateDatabaseBackup(ctx, inputs.DatabaseUUID, api.CreateDatabaseBackupJSONRequestBody{
		Frequency:                                inputs.Frequency,
		Enabled:                                  &inputs.Enabled,
		SaveS3:                                   &inputs.SaveS3,
		S3StorageUuid:                            coolify.PtrIfNonZero(inputs.S3StorageUUID),
		DatabasesToBackup:                        coolify.PtrIfNonZero(inputs.DatabasesToBackup),
		DumpAll:                                  &inputs.DumpAll,
		BackupNow:                                coolify.PtrIfNonZero(inputs.BackupNow),
		DatabaseBackupRetentionAmountLocally:     inputs.RetentionAmountLocally,
		DatabaseBackupRetentionDaysLocally:       inputs.RetentionDaysLocally,
		DatabaseBackupRetentionMaxStorageLocally: float32Ptr(inputs.RetentionMaxStorageLocally),
		DatabaseBackupRetentionAmountS3:          inputs.RetentionAmountS3,
		DatabaseBackupRetentionDaysS3:            inputs.RetentionDaysS3,
		DatabaseBackupRetentionMaxStorageS3:      float32Ptr(inputs.RetentionMaxStorageS3),
		Timeout:                                  coolify.PtrIfNonZero(inputs.Timeout),
	})
	if err != nil {
		return coolify.DatabaseBackup{}, err
	}
	return c.GetDatabaseBackup(ctx, inputs.DatabaseUUID, uuid)
}

// applyDatabaseBackup patches the fields of current that differ from the
// inputs. Coolify never reports the configured S3 storage, so the storage UUID
// is sent whenever it is unknown (previousS3 is nil, i.e. on adoption) or
// differs from the previous inputs, and whenever save_s3 is switched on because
// Coolify requires both together.
func applyDatabaseBackup(ctx context.Context, c *coolify.Client, current coolify.DatabaseBackup, inputs DatabaseBackupArgs, previousS3 *string) (coolify.DatabaseBackup, error) {
	var body api.UpdateDatabaseBackupJSONRequestBody
	var patch patch
	patch.str(&body.Frequency, inputs.Frequency, current.Frequency)
	patch.boolean(&body.Enabled, inputs.Enabled, current.Enabled)
	patch.boolean(&body.SaveS3, inputs.SaveS3, current.SaveS3)
	patch.boolean(&body.DumpAll, inputs.DumpAll, current.DumpAll)
	patch.str(&body.DatabasesToBackup, inputs.DatabasesToBackup, coolify.Deref(current.DatabasesToBackup))
	patch.optionalInt(&body.DatabaseBackupRetentionAmountLocally, inputs.RetentionAmountLocally, current.RetentionAmountLocally)
	patch.optionalInt(&body.DatabaseBackupRetentionDaysLocally, inputs.RetentionDaysLocally, current.RetentionDaysLocally)
	patch.optionalFloat(&body.DatabaseBackupRetentionMaxStorageLocally, inputs.RetentionMaxStorageLocally, current.RetentionMaxStorageLocally)
	patch.optionalInt(&body.DatabaseBackupRetentionAmountS3, inputs.RetentionAmountS3, current.RetentionAmountS3)
	patch.optionalInt(&body.DatabaseBackupRetentionDaysS3, inputs.RetentionDaysS3, current.RetentionDaysS3)
	patch.optionalFloat(&body.DatabaseBackupRetentionMaxStorageS3, inputs.RetentionMaxStorageS3, current.RetentionMaxStorageS3)
	patch.integer(&body.Timeout, inputs.Timeout, current.Timeout)
	if inputs.S3StorageUUID != "" {
		storageChanged := previousS3 == nil || *previousS3 != inputs.S3StorageUUID
		if storageChanged || (body.SaveS3 != nil && *body.SaveS3) {
			body.S3StorageUuid = &inputs.S3StorageUUID
			patch.changed = true
		}
	}
	if !patch.changed {
		return current, nil
	}
	if err := c.UpdateDatabaseBackup(ctx, inputs.DatabaseUUID, current.UUID, body); err != nil {
		return coolify.DatabaseBackup{}, err
	}
	return c.GetDatabaseBackup(ctx, inputs.DatabaseUUID, current.UUID)
}

// databaseBackupInputs derives the inputs from the configuration Coolify
// reports, keeping unmanaged optional inputs and the ones Coolify never reports.
func databaseBackupInputs(previous DatabaseBackupArgs, backup coolify.DatabaseBackup) DatabaseBackupArgs {
	inputs := previous
	inputs.Frequency = backup.Frequency
	inputs.Enabled = backup.Enabled
	inputs.SaveS3 = backup.SaveS3
	inputs.DumpAll = backup.DumpAll
	inputs.DatabasesToBackup = ifSet(previous.DatabasesToBackup, coolify.Deref(backup.DatabasesToBackup))
	inputs.RetentionAmountLocally = ifSetPtr(previous.RetentionAmountLocally, backup.RetentionAmountLocally)
	inputs.RetentionDaysLocally = ifSetPtr(previous.RetentionDaysLocally, backup.RetentionDaysLocally)
	inputs.RetentionMaxStorageLocally = ifSetPtr(previous.RetentionMaxStorageLocally, backup.RetentionMaxStorageLocally)
	inputs.RetentionAmountS3 = ifSetPtr(previous.RetentionAmountS3, backup.RetentionAmountS3)
	inputs.RetentionDaysS3 = ifSetPtr(previous.RetentionDaysS3, backup.RetentionDaysS3)
	inputs.RetentionMaxStorageS3 = ifSetPtr(previous.RetentionMaxStorageS3, backup.RetentionMaxStorageS3)
	inputs.Timeout = ifSet(previous.Timeout, coolify.Deref(backup.Timeout))
	return inputs
}

func databaseBackupState(inputs DatabaseBackupArgs, backup coolify.DatabaseBackup) DatabaseBackupState {
	return DatabaseBackupState{DatabaseBackupArgs: inputs, UUID: backup.UUID}
}
