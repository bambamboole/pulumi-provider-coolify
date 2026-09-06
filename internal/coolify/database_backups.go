package coolify

import (
	"context"
	"net/http"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// DatabaseBackup is a scheduled backup configuration of a standalone database.
// The OpenAPI specification declares the list endpoint as returning a plain
// string, so the model is hand-written from the JSON Coolify actually returns.
// Coolify reports the S3 storage as an internal integer id that the S3 storage
// API never exposes, so the configured storage cannot be resolved to a UUID.
type DatabaseBackup struct {
	UUID                       string   `json:"uuid"`
	Enabled                    bool     `json:"enabled"`
	SaveS3                     bool     `json:"save_s3"`
	S3StorageID                *int     `json:"s3_storage_id"`
	Frequency                  string   `json:"frequency"`
	DatabasesToBackup          *string  `json:"databases_to_backup"`
	DumpAll                    bool     `json:"dump_all"`
	RetentionAmountLocally     *int     `json:"database_backup_retention_amount_locally"`
	RetentionDaysLocally       *int     `json:"database_backup_retention_days_locally"`
	RetentionMaxStorageLocally *float64 `json:"database_backup_retention_max_storage_locally"`
	RetentionAmountS3          *int     `json:"database_backup_retention_amount_s3"`
	RetentionDaysS3            *int     `json:"database_backup_retention_days_s3"`
	RetentionMaxStorageS3      *float64 `json:"database_backup_retention_max_storage_s3"`
	Timeout                    *int     `json:"timeout"`
}

func (c *Client) ListDatabaseBackups(ctx context.Context, databaseUUID string) ([]DatabaseBackup, error) {
	return decode[[]DatabaseBackup](c.api.GetDatabaseBackupsByUuid(ctx, databaseUUID))
}

// GetDatabaseBackup finds a backup configuration by UUID. The API has no
// single-configuration endpoint, so it lists the database's configurations and
// returns a 404 APIError when missing.
func (c *Client) GetDatabaseBackup(ctx context.Context, databaseUUID, backupUUID string) (DatabaseBackup, error) {
	backups, err := c.ListDatabaseBackups(ctx, databaseUUID)
	if err != nil {
		return DatabaseBackup{}, err
	}
	for _, backup := range backups {
		if backup.UUID == backupUUID {
			return backup, nil
		}
	}
	return DatabaseBackup{}, &APIError{
		Status: http.StatusNotFound,
		Method: http.MethodGet,
		Path:   apiPath + "/databases/" + databaseUUID + "/backups/" + backupUUID,
		Body:   `{"message":"Backup configuration not found."}`,
	}
}

// CreateDatabaseBackup creates a backup configuration and returns its UUID.
func (c *Client) CreateDatabaseBackup(ctx context.Context, databaseUUID string, body api.CreateDatabaseBackupJSONRequestBody) (string, error) {
	return decodeUUID(c.api.CreateDatabaseBackup(ctx, databaseUUID, body))
}

func (c *Client) UpdateDatabaseBackup(ctx context.Context, databaseUUID, backupUUID string, body api.UpdateDatabaseBackupJSONRequestBody) error {
	return check(c.api.UpdateDatabaseBackup(ctx, databaseUUID, backupUUID, body))
}

// DeleteDatabaseBackup deletes the configuration and all its executions,
// including the files uploaded to S3 when deleteS3 is true.
func (c *Client) DeleteDatabaseBackup(ctx context.Context, databaseUUID, backupUUID string, deleteS3 bool) error {
	return check(c.api.DeleteBackupConfigurationByUuid(ctx, databaseUUID, backupUUID, &api.DeleteBackupConfigurationByUuidParams{DeleteS3: &deleteS3}))
}
