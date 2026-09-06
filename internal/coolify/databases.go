package coolify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// DatabaseType is one of the standalone database engines Coolify can run.
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypeMariaDB    DatabaseType = "mariadb"
	DatabaseTypeMongoDB    DatabaseType = "mongodb"
	DatabaseTypeRedis      DatabaseType = "redis"
	DatabaseTypeKeyDB      DatabaseType = "keydb"
	DatabaseTypeDragonfly  DatabaseType = "dragonfly"
	DatabaseTypeClickHouse DatabaseType = "clickhouse"
)

// DatabaseTypes lists all supported database types.
func DatabaseTypes() []DatabaseType {
	return []DatabaseType{
		DatabaseTypePostgreSQL, DatabaseTypeMySQL, DatabaseTypeMariaDB, DatabaseTypeMongoDB,
		DatabaseTypeRedis, DatabaseTypeKeyDB, DatabaseTypeDragonfly, DatabaseTypeClickHouse,
	}
}

// StandaloneType returns the database_type value Coolify reports for the type.
func (t DatabaseType) StandaloneType() string { return "standalone-" + string(t) }

// Database is a standalone database. The OpenAPI specification declares the
// database endpoints as returning plain strings, so the model is hand-written
// from the JSON Coolify actually returns.
type Database struct {
	UUID          string  `json:"uuid"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	DatabaseType  string  `json:"database_type"`
	Image         string  `json:"image"`
	IsPublic      bool    `json:"is_public"`
	PublicPort    *int    `json:"public_port"`
	EnvironmentID int     `json:"environment_id"`
	Status        string  `json:"status"`
	InternalDBURL *string `json:"internal_db_url"`
	ExternalDBURL *string `json:"external_db_url"`

	PostgresUser     *string `json:"postgres_user"`
	PostgresPassword *string `json:"postgres_password"`
	PostgresDB       *string `json:"postgres_db"`

	MysqlUser         *string `json:"mysql_user"`
	MysqlPassword     *string `json:"mysql_password"`
	MysqlDatabase     *string `json:"mysql_database"`
	MysqlRootPassword *string `json:"mysql_root_password"`

	MariadbUser         *string `json:"mariadb_user"`
	MariadbPassword     *string `json:"mariadb_password"`
	MariadbDatabase     *string `json:"mariadb_database"`
	MariadbRootPassword *string `json:"mariadb_root_password"`

	MongoInitdbRootUsername *string `json:"mongo_initdb_root_username"`
	MongoInitdbRootPassword *string `json:"mongo_initdb_root_password"`
	MongoInitdbDatabase     *string `json:"mongo_initdb_database"`

	RedisPassword     *string `json:"redis_password"`
	KeydbPassword     *string `json:"keydb_password"`
	DragonflyPassword *string `json:"dragonfly_password"`

	ClickhouseAdminUser     *string `json:"clickhouse_admin_user"`
	ClickhouseAdminPassword *string `json:"clickhouse_admin_password"`
}

// Type derives the DatabaseType from the database_type field.
func (d Database) Type() DatabaseType {
	return DatabaseType(strings.TrimPrefix(d.DatabaseType, "standalone-"))
}

// Credentials returns the primary username, password and database name for the
// database engine. Engines without a concept return empty strings.
func (d Database) Credentials() (user, password, name string) {
	switch d.Type() {
	case DatabaseTypePostgreSQL:
		return Deref(d.PostgresUser), Deref(d.PostgresPassword), Deref(d.PostgresDB)
	case DatabaseTypeMySQL:
		return Deref(d.MysqlUser), Deref(d.MysqlPassword), Deref(d.MysqlDatabase)
	case DatabaseTypeMariaDB:
		return Deref(d.MariadbUser), Deref(d.MariadbPassword), Deref(d.MariadbDatabase)
	case DatabaseTypeMongoDB:
		return Deref(d.MongoInitdbRootUsername), Deref(d.MongoInitdbRootPassword), Deref(d.MongoInitdbDatabase)
	case DatabaseTypeRedis:
		return "", Deref(d.RedisPassword), ""
	case DatabaseTypeKeyDB:
		return "", Deref(d.KeydbPassword), ""
	case DatabaseTypeDragonfly:
		return "", Deref(d.DragonflyPassword), ""
	case DatabaseTypeClickHouse:
		return Deref(d.ClickhouseAdminUser), Deref(d.ClickhouseAdminPassword), ""
	}
	return "", "", ""
}

// CreateDatabaseInput holds the fields shared by all database create endpoints.
type CreateDatabaseInput struct {
	ServerUUID      string  `json:"server_uuid"`
	ProjectUUID     string  `json:"project_uuid"`
	EnvironmentName string  `json:"environment_name"`
	EnvironmentUUID string  `json:"environment_uuid,omitempty"`
	Name            string  `json:"name"`
	Description     *string `json:"description,omitempty"`
	Image           *string `json:"image,omitempty"`
	IsPublic        bool    `json:"is_public"`
	PublicPort      *int    `json:"public_port,omitempty"`
	InstantDeploy   bool    `json:"instant_deploy"`
}

type bodyRequest func(context.Context, string, io.Reader, ...api.RequestEditorFn) (*http.Response, error)

// CreateDatabase creates a database of the given type and returns its UUID.
func (c *Client) CreateDatabase(ctx context.Context, typ DatabaseType, in CreateDatabaseInput) (string, error) {
	var create bodyRequest
	switch typ {
	case DatabaseTypePostgreSQL:
		create = c.api.CreateDatabasePostgresqlWithBody
	case DatabaseTypeMySQL:
		create = c.api.CreateDatabaseMysqlWithBody
	case DatabaseTypeMariaDB:
		create = c.api.CreateDatabaseMariadbWithBody
	case DatabaseTypeMongoDB:
		create = c.api.CreateDatabaseMongodbWithBody
	case DatabaseTypeRedis:
		create = c.api.CreateDatabaseRedisWithBody
	case DatabaseTypeKeyDB:
		create = c.api.CreateDatabaseKeydbWithBody
	case DatabaseTypeDragonfly:
		create = c.api.CreateDatabaseDragonflyWithBody
	case DatabaseTypeClickHouse:
		create = c.api.CreateDatabaseClickhouseWithBody
	default:
		return "", fmt.Errorf("coolify: unsupported database type %q", typ)
	}
	body, err := json.Marshal(in)
	if err != nil {
		return "", fmt.Errorf("coolify: encode database: %w", err)
	}
	return decodeUUID(create(ctx, "application/json", bytes.NewReader(body)))
}

func (c *Client) ListDatabases(ctx context.Context) ([]Database, error) {
	return decode[[]Database](c.api.ListDatabases(ctx))
}

func (c *Client) GetDatabase(ctx context.Context, uuid string) (Database, error) {
	return decode[Database](c.api.GetDatabaseByUuid(ctx, uuid))
}

func (c *Client) UpdateDatabase(ctx context.Context, uuid string, body api.UpdateDatabaseByUuidJSONRequestBody) error {
	return check(c.api.UpdateDatabaseByUuid(ctx, uuid, body))
}

func (c *Client) DeleteDatabase(ctx context.Context, uuid string) error {
	return check(c.api.DeleteDatabaseByUuid(ctx, uuid, nil))
}

// MoveDatabase moves the database into another environment, possibly of
// another project. Coolify only re-parents the record; containers keep running.
func (c *Client) MoveDatabase(ctx context.Context, uuid, environmentUUID string) error {
	return check(c.api.MoveDatabaseByUuid(ctx, uuid, api.MoveDatabaseByUuidJSONRequestBody{EnvironmentUuid: environmentUUID}))
}
