package coolify

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// VolumeBackup is the schedule Coolify reports after an upsert. The API has no
// endpoint to read a schedule back.
type VolumeBackup = api.VolumeBackupScheduleResponse

// SetVolumeBackup creates or replaces the backup schedule of a storage. The
// body is total: fields left nil fall back to Coolify's defaults.
func (c *Client) SetVolumeBackup(ctx context.Context, owner Owner, storageUUID string, body api.VolumeBackupScheduleRequest) (VolumeBackup, error) {
	var resp *http.Response
	var err error
	switch owner.Kind {
	case OwnerApplication:
		resp, err = c.api.SetApplicationStorageBackupSchedule(ctx, owner.UUID, storageUUID, body)
	case OwnerDatabase:
		resp, err = c.api.SetDatabaseStorageBackupSchedule(ctx, owner.UUID, storageUUID, body)
	case OwnerService:
		resp, err = c.api.SetServiceStorageBackupSchedule(ctx, owner.UUID, storageUUID, body)
	default:
		return VolumeBackup{}, fmt.Errorf("coolify: unsupported storage owner %q", owner.Kind)
	}
	return decode[VolumeBackup](resp, err)
}

// RunVolumeBackup queues an immediate backup of a storage that has a schedule.
func (c *Client) RunVolumeBackup(ctx context.Context, owner Owner, storageUUID string) error {
	switch owner.Kind {
	case OwnerApplication:
		return check(c.api.RunApplicationStorageBackup(ctx, owner.UUID, storageUUID))
	case OwnerDatabase:
		return check(c.api.RunDatabaseStorageBackup(ctx, owner.UUID, storageUUID))
	case OwnerService:
		return check(c.api.RunServiceStorageBackup(ctx, owner.UUID, storageUUID))
	default:
		return fmt.Errorf("coolify: unsupported storage owner %q", owner.Kind)
	}
}

// DeleteVolumeBackup deletes the schedule of a storage together with all its
// local and S3 archives. Coolify answers 409 while a backup is running.
func (c *Client) DeleteVolumeBackup(ctx context.Context, owner Owner, storageUUID string) error {
	switch owner.Kind {
	case OwnerApplication:
		return check(c.api.DeleteApplicationStorageBackupSchedule(ctx, owner.UUID, storageUUID))
	case OwnerDatabase:
		return check(c.api.DeleteDatabaseStorageBackupSchedule(ctx, owner.UUID, storageUUID))
	case OwnerService:
		return check(c.api.DeleteServiceStorageBackupSchedule(ctx, owner.UUID, storageUUID))
	default:
		return fmt.Errorf("coolify: unsupported storage owner %q", owner.Kind)
	}
}
