package provider

import (
	"context"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// VolumeBackup manages the backup schedule of a persistent volume or directory
// mount of an application, database or service.
type VolumeBackup struct{}

type VolumeBackupArgs struct {
	// UUID of the application owning the storage.
	ApplicationUUID string `pulumi:"applicationUuid,optional"`
	// UUID of the database owning the storage.
	DatabaseUUID string `pulumi:"databaseUuid,optional"`
	// UUID of the service owning the storage.
	ServiceUUID string `pulumi:"serviceUuid,optional"`
	// UUID of the storage to back up.
	StorageUUID string `pulumi:"storageUuid,optional"`
	// Mount path of the storage to back up, resolved on the owner.
	MountPath string `pulumi:"mountPath,optional"`
	// Volume name to disambiguate the mount path.
	VolumeName string `pulumi:"volumeName,optional"`

	// Cron expression or Coolify shorthand.
	Frequency string `pulumi:"frequency"`
	// Whether the schedule is enabled.
	Enabled bool `pulumi:"enabled,optional"`
	// Upload archives to an S3 storage.
	SaveS3 bool `pulumi:"saveS3,optional"`
	// UUID of the S3 storage. Required when saveS3 is true.
	S3StorageUUID string `pulumi:"s3StorageUuid,optional"`
	// Keep no local archive once uploaded to S3.
	DisableLocalBackup bool `pulumi:"disableLocalBackup,optional"`
	// Stop the containers using the storage while the archive is taken.
	StopDuringBackup bool `pulumi:"stopDuringBackup,optional"`
	// Number of archives kept locally; 0 keeps all.
	RetentionAmountLocally int `pulumi:"retentionAmountLocally,optional"`
	// Days local archives are kept; 0 keeps them forever.
	RetentionDaysLocally int `pulumi:"retentionDaysLocally,optional"`
	// Maximum local storage in GB; 0 is unlimited.
	RetentionMaxStorageLocally float64 `pulumi:"retentionMaxStorageLocally,optional"`
	// Number of archives kept in S3; 0 keeps all.
	RetentionAmountS3 int `pulumi:"retentionAmountS3,optional"`
	// Days S3 archives are kept; 0 keeps them forever.
	RetentionDaysS3 int `pulumi:"retentionDaysS3,optional"`
	// Maximum S3 storage in GB; 0 is unlimited.
	RetentionMaxStorageS3 float64 `pulumi:"retentionMaxStorageS3,optional"`
	// Timeout of a backup run in seconds (60 to 36000).
	Timeout int `pulumi:"timeout,optional"`
	// Run a backup right after creating the schedule.
	BackupNow bool `pulumi:"backupNow,optional"`
}

type VolumeBackupState struct {
	VolumeBackupArgs
	// UUID of the schedule in Coolify.
	UUID string `pulumi:"uuid"`
	// UUID of the storage the schedule belongs to.
	ResolvedStorageUUID string `pulumi:"resolvedStorageUuid"`
	// Storage type: persistent or directory.
	StorageType string `pulumi:"storageType"`
}

func (r *VolumeBackup) Annotate(a infer.Annotator) {
	a.SetToken("index", "VolumeBackup")
	a.Describe(&r, "The backup schedule of a persistent volume or directory mount of a Coolify application, database or service, archived locally and optionally uploaded to an S3 storage. Each storage has at most one schedule, which is created or replaced as a whole. Coolify offers no endpoint to read a schedule back, so changes made in the Coolify UI are not detected. Destroying the resource deletes the schedule together with all local and S3 archives; use retainOnDelete to keep them.")
}

func (args *VolumeBackupArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ApplicationUUID, "UUID of the application owning the storage. Exactly one of applicationUuid, databaseUuid and serviceUuid must be set. Changing it replaces the schedule.")
	a.Describe(&args.DatabaseUUID, "UUID of the database owning the storage. Exactly one of applicationUuid, databaseUuid and serviceUuid must be set. Changing it replaces the schedule.")
	a.Describe(&args.ServiceUUID, "UUID of the service owning the storage. Exactly one of applicationUuid, databaseUuid and serviceUuid must be set. Changing it replaces the schedule.")
	a.Describe(&args.StorageUUID, "UUID of the storage to back up (the uuid output of a Storage resource or of getStorage). Exactly one of storageUuid and mountPath must be set. Changing it replaces the schedule.")
	a.Describe(&args.MountPath, "Mount path of the storage to back up, resolved on the owner. Exactly one of storageUuid and mountPath must be set. Changing it replaces the schedule.")
	a.Describe(&args.VolumeName, "Volume name to disambiguate the mount path when several containers of a service mount the same path. Changing it replaces the schedule.")
	a.Describe(&args.Frequency, `Cron expression or Coolify shorthand such as "daily" or "0 3 * * *".`)
	a.Describe(&args.Enabled, "Whether the schedule is enabled.")
	a.Describe(&args.SaveS3, "Upload archives to an S3 storage.")
	a.Describe(&args.S3StorageUUID, "UUID of the S3 storage (the uuid output of an S3Storage resource). Required when saveS3 is true; the storage must be validated as usable by Coolify.")
	a.Describe(&args.DisableLocalBackup, "Keep no local archive once uploaded to S3. Requires saveS3.")
	a.Describe(&args.StopDuringBackup, "Stop the containers using the storage while the archive is taken, for consistent snapshots of databases inside services.")
	a.Describe(&args.RetentionAmountLocally, "Number of archives kept locally; 0 keeps all.")
	a.Describe(&args.RetentionDaysLocally, "Days local archives are kept; 0 keeps them forever.")
	a.Describe(&args.RetentionMaxStorageLocally, "Maximum local storage in GB; 0 is unlimited.")
	a.Describe(&args.RetentionAmountS3, "Number of archives kept in S3; 0 keeps all.")
	a.Describe(&args.RetentionDaysS3, "Days S3 archives are kept; 0 keeps them forever.")
	a.Describe(&args.RetentionMaxStorageS3, "Maximum S3 storage in GB; 0 is unlimited.")
	a.Describe(&args.Timeout, "Timeout of a backup run in seconds, between 60 and 36000. Leave unset to keep Coolify's default.")
	a.Describe(&args.BackupNow, "Run a backup right after creating the schedule. Only relevant on create.")
	a.SetDefault(&args.Enabled, true)
	a.SetDefault(&args.RetentionAmountLocally, 7)
	a.SetDefault(&args.RetentionAmountS3, 7)
}

func (state *VolumeBackupState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the schedule in Coolify.")
	a.Describe(&state.ResolvedStorageUUID, "UUID of the storage the schedule belongs to.")
	a.Describe(&state.StorageType, "Storage type: persistent or directory.")
}

func (VolumeBackup) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[VolumeBackupArgs], error) {
	args, failures, err := infer.DefaultCheck[VolumeBackupArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[VolumeBackupArgs]{}, err
	}
	if _, err := storageOwner(args.ApplicationUUID, args.DatabaseUUID, args.ServiceUUID); err != nil {
		failures = append(failures, p.CheckFailure{Property: "applicationUuid", Reason: err.Error()})
	}
	if (args.StorageUUID == "") == (args.MountPath == "") {
		failures = append(failures, p.CheckFailure{Property: "storageUuid", Reason: "exactly one of storageUuid and mountPath must be set"})
	}
	if args.SaveS3 && args.S3StorageUUID == "" {
		failures = append(failures, p.CheckFailure{Property: "s3StorageUuid", Reason: "s3StorageUuid is required when saveS3 is true"})
	}
	if args.DisableLocalBackup && !args.SaveS3 {
		failures = append(failures, p.CheckFailure{Property: "disableLocalBackup", Reason: "disableLocalBackup requires saveS3"})
	}
	return infer.CheckResponse[VolumeBackupArgs]{Inputs: args, Failures: failures}, nil
}

func (VolumeBackup) Create(ctx context.Context, req infer.CreateRequest[VolumeBackupArgs]) (infer.CreateResponse[VolumeBackupState], error) {
	if req.DryRun {
		return infer.CreateResponse[VolumeBackupState]{Output: VolumeBackupState{VolumeBackupArgs: req.Inputs}}, nil
	}
	c := client(ctx)
	owner, storageUUID, err := resolveVolumeBackupTarget(ctx, c, req.Inputs)
	if err != nil {
		return infer.CreateResponse[VolumeBackupState]{}, err
	}
	backup, err := c.SetVolumeBackup(ctx, owner, storageUUID, volumeBackupBody(req.Inputs))
	if err != nil {
		return infer.CreateResponse[VolumeBackupState]{}, err
	}
	if req.Inputs.BackupNow {
		if err := c.RunVolumeBackup(ctx, owner, storageUUID); err != nil {
			return infer.CreateResponse[VolumeBackupState]{}, err
		}
	}
	return infer.CreateResponse[VolumeBackupState]{ID: backup.Uuid, Output: volumeBackupState(req.Inputs, backup)}, nil
}

func (VolumeBackup) Diff(ctx context.Context, req infer.DiffRequest[VolumeBackupArgs, VolumeBackupState]) (infer.DiffResponse, error) {
	diff := diffArgs(req.State.VolumeBackupArgs, req.Inputs,
		"applicationUuid", "databaseUuid", "serviceUuid", "storageUuid", "mountPath", "volumeName")
	// Only relevant on create.
	delete(diff, "backupNow")
	// A replacement always targets another storage, whose schedule is
	// independent of the one being deleted.
	return diffResponse(diff, false), nil
}

func (VolumeBackup) Update(ctx context.Context, req infer.UpdateRequest[VolumeBackupArgs, VolumeBackupState]) (infer.UpdateResponse[VolumeBackupState], error) {
	if req.DryRun {
		state := req.State
		state.VolumeBackupArgs = req.Inputs
		return infer.UpdateResponse[VolumeBackupState]{Output: state}, nil
	}
	owner, err := storageOwner(req.Inputs.ApplicationUUID, req.Inputs.DatabaseUUID, req.Inputs.ServiceUUID)
	if err != nil {
		return infer.UpdateResponse[VolumeBackupState]{}, err
	}
	backup, err := client(ctx).SetVolumeBackup(ctx, owner, req.State.ResolvedStorageUUID, volumeBackupBody(req.Inputs))
	if err != nil {
		return infer.UpdateResponse[VolumeBackupState]{}, err
	}
	return infer.UpdateResponse[VolumeBackupState]{Output: volumeBackupState(req.Inputs, backup)}, nil
}

// Read cannot fetch the schedule because Coolify has no endpoint for it. It
// verifies that the storage still exists and otherwise keeps the stored inputs.
func (VolumeBackup) Read(ctx context.Context, req infer.ReadRequest[VolumeBackupArgs, VolumeBackupState]) (infer.ReadResponse[VolumeBackupArgs, VolumeBackupState], error) {
	state := req.State
	if state.ResolvedStorageUUID == "" {
		return infer.ReadResponse[VolumeBackupArgs, VolumeBackupState]{}, fmt.Errorf("volume backup %s has no resolved storage in its state and cannot be read", req.ID)
	}
	owner, err := storageOwner(state.ApplicationUUID, state.DatabaseUUID, state.ServiceUUID)
	if err != nil {
		return infer.ReadResponse[VolumeBackupArgs, VolumeBackupState]{}, err
	}
	storage, err := client(ctx).GetStorage(ctx, owner, state.ResolvedStorageUUID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[VolumeBackupArgs, VolumeBackupState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[VolumeBackupArgs, VolumeBackupState]{}, err
	}
	state.StorageType = string(storage.Type())
	return infer.ReadResponse[VolumeBackupArgs, VolumeBackupState]{ID: req.ID, Inputs: state.VolumeBackupArgs, State: state}, nil
}

func (VolumeBackup) Delete(ctx context.Context, req infer.DeleteRequest[VolumeBackupState]) (infer.DeleteResponse, error) {
	owner, err := storageOwner(req.State.ApplicationUUID, req.State.DatabaseUUID, req.State.ServiceUUID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	err = client(ctx).DeleteVolumeBackup(ctx, owner, req.State.ResolvedStorageUUID)
	if err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

// resolveVolumeBackupTarget returns the owner and the UUID of the storage to
// back up, looking it up by mount path when no UUID was given.
func resolveVolumeBackupTarget(ctx context.Context, c *coolify.Client, inputs VolumeBackupArgs) (coolify.Owner, string, error) {
	owner, err := storageOwner(inputs.ApplicationUUID, inputs.DatabaseUUID, inputs.ServiceUUID)
	if err != nil {
		return coolify.Owner{}, "", err
	}
	if inputs.StorageUUID != "" {
		return owner, inputs.StorageUUID, nil
	}
	storage, err := findStorage(ctx, c, owner, inputs.MountPath, inputs.VolumeName)
	if err != nil {
		return coolify.Owner{}, "", err
	}
	return owner, storage.UUID, nil
}

// volumeBackupBody builds the complete upsert body. Coolify resets omitted
// fields to its defaults, so every input is always sent.
func volumeBackupBody(inputs VolumeBackupArgs) api.VolumeBackupScheduleRequest {
	return api.VolumeBackupScheduleRequest{
		Frequency:                  inputs.Frequency,
		Enabled:                    &inputs.Enabled,
		SaveS3:                     &inputs.SaveS3,
		S3StorageUuid:              coolify.PtrIfNonZero(inputs.S3StorageUUID),
		DisableLocalBackup:         &inputs.DisableLocalBackup,
		StopDuringBackup:           &inputs.StopDuringBackup,
		RetentionAmountLocally:     &inputs.RetentionAmountLocally,
		RetentionDaysLocally:       &inputs.RetentionDaysLocally,
		RetentionMaxStorageLocally: coolify.Ptr(float32(inputs.RetentionMaxStorageLocally)),
		RetentionAmountS3:          &inputs.RetentionAmountS3,
		RetentionDaysS3:            &inputs.RetentionDaysS3,
		RetentionMaxStorageS3:      coolify.Ptr(float32(inputs.RetentionMaxStorageS3)),
		Timeout:                    coolify.PtrIfNonZero(inputs.Timeout),
	}
}

func volumeBackupState(inputs VolumeBackupArgs, backup coolify.VolumeBackup) VolumeBackupState {
	return VolumeBackupState{
		VolumeBackupArgs:    inputs,
		UUID:                backup.Uuid,
		ResolvedStorageUUID: backup.StorageUuid,
		StorageType:         string(backup.StorageType),
	}
}
