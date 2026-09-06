package provider

import (
	"context"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// DatabaseType is the standalone database engine to run.
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = DatabaseType(coolify.DatabaseTypePostgreSQL)
	DatabaseTypeMySQL      DatabaseType = DatabaseType(coolify.DatabaseTypeMySQL)
	DatabaseTypeMariaDB    DatabaseType = DatabaseType(coolify.DatabaseTypeMariaDB)
	DatabaseTypeMongoDB    DatabaseType = DatabaseType(coolify.DatabaseTypeMongoDB)
	DatabaseTypeRedis      DatabaseType = DatabaseType(coolify.DatabaseTypeRedis)
	DatabaseTypeKeyDB      DatabaseType = DatabaseType(coolify.DatabaseTypeKeyDB)
	DatabaseTypeDragonfly  DatabaseType = DatabaseType(coolify.DatabaseTypeDragonfly)
	DatabaseTypeClickHouse DatabaseType = DatabaseType(coolify.DatabaseTypeClickHouse)
)

func (DatabaseType) Values() []infer.EnumValue[DatabaseType] {
	return []infer.EnumValue[DatabaseType]{
		{Name: "PostgreSQL", Value: DatabaseTypePostgreSQL, Description: "PostgreSQL"},
		{Name: "MySQL", Value: DatabaseTypeMySQL, Description: "MySQL"},
		{Name: "MariaDB", Value: DatabaseTypeMariaDB, Description: "MariaDB"},
		{Name: "MongoDB", Value: DatabaseTypeMongoDB, Description: "MongoDB"},
		{Name: "Redis", Value: DatabaseTypeRedis, Description: "Redis"},
		{Name: "KeyDB", Value: DatabaseTypeKeyDB, Description: "KeyDB"},
		{Name: "Dragonfly", Value: DatabaseTypeDragonfly, Description: "Dragonfly"},
		{Name: "ClickHouse", Value: DatabaseTypeClickHouse, Description: "ClickHouse"},
	}
}

// Database manages a standalone database inside a Coolify project environment.
type Database struct{}

type DatabaseArgs struct {
	// Database engine.
	Type DatabaseType `pulumi:"type"`
	// Name of the database. An existing database with this name in the same
	// environment is adopted.
	Name string `pulumi:"name"`
	// Description of the database.
	Description string `pulumi:"description,optional"`
	// Container image, e.g. "postgres:17-alpine". Leave unset to use Coolify's default.
	Image string `pulumi:"image,optional"`
	// Whether the database is exposed publicly.
	IsPublic bool `pulumi:"isPublic,optional"`
	// Public port to expose. Required when isPublic is true.
	PublicPort *int `pulumi:"publicPort,optional"`
	// Start the database right after creating it.
	InstantDeploy bool `pulumi:"instantDeploy,optional"`
	// UUID of the Coolify project the database belongs to.
	ProjectUUID string `pulumi:"projectUuid"`
	// Name of the environment inside the project.
	EnvironmentName string `pulumi:"environmentName"`
	// UUID of the server hosting the database.
	ServerUUID string `pulumi:"serverUuid"`
	// Tags attached to the database in addition to the provider's default tags.
	Tags []string `pulumi:"tags,optional"`
}

type DatabaseState struct {
	DatabaseArgs
	// Tags the provider attached: the provider's default tags plus the declared ones.
	AppliedTags []string `pulumi:"appliedTags"`
	// UUID of the database in Coolify.
	UUID string `pulumi:"uuid"`
	// ID of the Coolify environment the database lives in.
	EnvironmentID int `pulumi:"environmentId"`
	// Status reported by Coolify.
	Status string `pulumi:"status"`
	// Connection URL for resources on the same Docker network.
	InternalURL string `pulumi:"internalUrl" provider:"secret"`
	// Public connection URL, when exposed.
	ExternalURL string `pulumi:"externalUrl" provider:"secret"`
	// Username of the primary database user.
	Username string `pulumi:"username"`
	// Password of the primary database user.
	Password string `pulumi:"password" provider:"secret"`
	// Name of the default database, for engines that have one.
	DatabaseName string `pulumi:"databaseName"`
}

func (r *Database) Annotate(a infer.Annotator) {
	a.SetToken("index", "Database")
	a.Describe(&r, "A standalone Coolify database (PostgreSQL, MySQL, MariaDB, MongoDB, Redis, KeyDB, Dragonfly, ClickHouse) inside a project environment. An existing database with the same name in the environment is adopted on create.")
}

func (args *DatabaseArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Type, "Database engine.")
	a.Describe(&args.Name, "Name of the database. An existing database with this name in the same environment is adopted.")
	a.Describe(&args.Description, "Description of the database.")
	a.Describe(&args.Image, `Container image, e.g. "postgres:17-alpine". Leave unset to keep Coolify's default.`)
	a.Describe(&args.IsPublic, "Whether the database is exposed publicly.")
	a.Describe(&args.PublicPort, "Public port to expose. Required when isPublic is true.")
	a.Describe(&args.InstantDeploy, "Start the database right after creating it.")
	a.Describe(&args.ProjectUUID, "UUID of the Coolify project (the uuid output of a Project resource). Changing it moves the resource to the new project in place.")
	a.Describe(&args.EnvironmentName, "Name of the environment inside the project. Changing it moves the resource in place; the environment must already exist.")
	a.Describe(&args.ServerUUID, "UUID of the server hosting the database (the uuid output of a Server resource).")
	a.Describe(&args.Tags, "Tags attached to the database in addition to the provider's default tags. Declared tags are attached, tags removed from the declaration are detached, tags added in the Coolify UI are left untouched.")
}

func (state *DatabaseState) Annotate(a infer.Annotator) {
	a.Describe(&state.AppliedTags, "Tags the provider attached: the provider's default tags plus the declared ones.")
	a.Describe(&state.UUID, "UUID of the database in Coolify.")
	a.Describe(&state.EnvironmentID, "ID of the Coolify environment the database lives in.")
	a.Describe(&state.Status, "Status reported by Coolify.")
	a.Describe(&state.InternalURL, "Connection URL for resources on the same Docker network.")
	a.Describe(&state.ExternalURL, "Public connection URL, when exposed.")
	a.Describe(&state.Username, "Username of the primary database user.")
	a.Describe(&state.Password, "Password of the primary database user.")
	a.Describe(&state.DatabaseName, "Name of the default database, for engines that have one.")
}

func (Database) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[DatabaseArgs], error) {
	args, failures, err := infer.DefaultCheck[DatabaseArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[DatabaseArgs]{}, err
	}
	if args.IsPublic && args.PublicPort == nil {
		failures = append(failures, p.CheckFailure{Property: "publicPort", Reason: "publicPort is required when isPublic is true"})
	}
	failures = append(failures, checkTags("tags", args.Tags)...)
	args.Tags = normalizeTags(args.Tags)
	return infer.CheckResponse[DatabaseArgs]{Inputs: args, Failures: failures}, nil
}

func (Database) Create(ctx context.Context, req infer.CreateRequest[DatabaseArgs]) (infer.CreateResponse[DatabaseState], error) {
	if req.DryRun {
		return infer.CreateResponse[DatabaseState]{Output: DatabaseState{DatabaseArgs: req.Inputs}}, nil
	}
	c := client(ctx)
	database, err := createDatabase(ctx, c, req.Inputs)
	if err != nil {
		return infer.CreateResponse[DatabaseState]{}, err
	}
	state := databaseState(req.Inputs, database)
	if state.AppliedTags, err = reconcileTags(ctx, c, databaseOwner(state.UUID), effectiveTags(ctx, req.Inputs.Tags), nil); err != nil {
		return infer.CreateResponse[DatabaseState]{}, err
	}
	return infer.CreateResponse[DatabaseState]{ID: state.UUID, Output: state}, nil
}

func (Database) Diff(ctx context.Context, req infer.DiffRequest[DatabaseArgs, DatabaseState]) (infer.DiffResponse, error) {
	// Project and environment changes move the database in place.
	diff := diffArgs(req.State.DatabaseArgs, req.Inputs, "type", "serverUuid")
	// Only relevant on create.
	delete(diff, "instantDeploy")
	delete(diff, "tags")
	if tagsDiffer(effectiveTags(ctx, req.Inputs.Tags), req.State.AppliedTags) {
		diff["tags"] = p.PropertyDiff{Kind: p.Update}
	}
	return diffResponse(diff, req.State.Name == req.Inputs.Name), nil
}

func (Database) Update(ctx context.Context, req infer.UpdateRequest[DatabaseArgs, DatabaseState]) (infer.UpdateResponse[DatabaseState], error) {
	if req.DryRun {
		state := req.State
		state.DatabaseArgs = req.Inputs
		return infer.UpdateResponse[DatabaseState]{Output: state}, nil
	}
	c := client(ctx)
	current, err := c.GetDatabase(ctx, req.ID)
	if err != nil {
		return infer.UpdateResponse[DatabaseState]{}, err
	}
	moved, err := ensurePlacement(ctx, c, databasePlacement(req.State.DatabaseArgs), databasePlacement(req.Inputs),
		current.EnvironmentID, func(ctx context.Context, environmentUUID string) error {
			return c.MoveDatabase(ctx, req.ID, environmentUUID)
		})
	if err != nil {
		return infer.UpdateResponse[DatabaseState]{}, err
	}
	if moved {
		if current, err = c.GetDatabase(ctx, req.ID); err != nil {
			return infer.UpdateResponse[DatabaseState]{}, err
		}
	}
	database, err := applyDatabase(ctx, c, current, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[DatabaseState]{}, err
	}
	state := databaseState(req.Inputs, database)
	if state.AppliedTags, err = reconcileTags(ctx, c, databaseOwner(req.ID), effectiveTags(ctx, req.Inputs.Tags), req.State.AppliedTags); err != nil {
		return infer.UpdateResponse[DatabaseState]{}, err
	}
	return infer.UpdateResponse[DatabaseState]{Output: state}, nil
}

func (Database) Read(ctx context.Context, req infer.ReadRequest[DatabaseArgs, DatabaseState]) (infer.ReadResponse[DatabaseArgs, DatabaseState], error) {
	c := client(ctx)
	database, err := c.GetDatabase(ctx, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[DatabaseArgs, DatabaseState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[DatabaseArgs, DatabaseState]{}, err
	}
	inputs := databaseInputs(req.Inputs, database)
	tags, applied, err := readTags(ctx, c, databaseOwner(req.ID), req.Inputs.Tags, req.State.AppliedTags)
	if err != nil {
		return infer.ReadResponse[DatabaseArgs, DatabaseState]{}, err
	}
	inputs.Tags = tags
	state := databaseState(inputs, database)
	state.AppliedTags = applied
	return infer.ReadResponse[DatabaseArgs, DatabaseState]{ID: req.ID, Inputs: inputs, State: state}, nil
}

func (Database) Delete(ctx context.Context, req infer.DeleteRequest[DatabaseState]) (infer.DeleteResponse, error) {
	if err := client(ctx).DeleteDatabase(ctx, req.ID); err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func databaseOwner(uuid string) coolify.Owner {
	return coolify.Owner{Kind: coolify.OwnerDatabase, UUID: uuid}
}

func databasePlacement(args DatabaseArgs) placement {
	return placement{ProjectUUID: args.ProjectUUID, EnvironmentName: args.EnvironmentName}
}

// resolveEnvironment looks up the environment by name in the project.
func resolveEnvironment(ctx context.Context, c *coolify.Client, projectUUID, name string) (coolify.Environment, error) {
	environment, err := c.GetEnvironment(ctx, projectUUID, name)
	if coolify.IsNotFound(err) {
		return coolify.Environment{}, fmt.Errorf("coolify environment %q not found in project %q", name, projectUUID)
	}
	return environment, err
}

// createDatabase adopts the database with the same name in the environment or
// creates it, and reconciles its settings with the inputs.
func createDatabase(ctx context.Context, c *coolify.Client, inputs DatabaseArgs) (coolify.Database, error) {
	environment, err := resolveEnvironment(ctx, c, inputs.ProjectUUID, inputs.EnvironmentName)
	if err != nil {
		return coolify.Database{}, err
	}
	databases, err := c.ListDatabases(ctx)
	if err != nil {
		return coolify.Database{}, err
	}
	for _, candidate := range databases {
		if candidate.Name != inputs.Name || candidate.EnvironmentID != environment.ID {
			continue
		}
		if candidate.Type() != coolify.DatabaseType(inputs.Type) {
			return coolify.Database{}, fmt.Errorf("coolify database %q already exists in environment %q with type %q, expected %q",
				inputs.Name, inputs.EnvironmentName, candidate.Type(), inputs.Type)
		}
		return applyDatabase(ctx, c, candidate, inputs)
	}

	uuid, err := c.CreateDatabase(ctx, coolify.DatabaseType(inputs.Type), coolify.CreateDatabaseInput{
		ServerUUID:      inputs.ServerUUID,
		ProjectUUID:     inputs.ProjectUUID,
		EnvironmentName: environment.Name,
		EnvironmentUUID: environment.UUID,
		Name:            inputs.Name,
		Description:     coolify.PtrIfNonZero(inputs.Description),
		Image:           coolify.PtrIfNonZero(inputs.Image),
		IsPublic:        inputs.IsPublic,
		PublicPort:      inputs.PublicPort,
		InstantDeploy:   inputs.InstantDeploy,
	})
	if err != nil {
		return coolify.Database{}, err
	}
	return c.GetDatabase(ctx, uuid)
}

// applyDatabase patches the fields of current that differ from the inputs and
// returns the refreshed database.
func applyDatabase(ctx context.Context, c *coolify.Client, current coolify.Database, inputs DatabaseArgs) (coolify.Database, error) {
	var body api.UpdateDatabaseByUuidJSONRequestBody
	var patch patch
	patch.str(&body.Name, inputs.Name, current.Name)
	patch.text(&body.Description, inputs.Description, coolify.Deref(current.Description))
	patch.str(&body.Image, inputs.Image, current.Image)
	patch.boolean(&body.IsPublic, inputs.IsPublic, current.IsPublic)
	patch.optionalInt(&body.PublicPort, inputs.PublicPort, current.PublicPort)
	if !patch.changed {
		return current, nil
	}
	if err := c.UpdateDatabase(ctx, current.UUID, body); err != nil {
		return coolify.Database{}, err
	}
	return c.GetDatabase(ctx, current.UUID)
}

// databaseInputs derives the inputs from the database Coolify reports, keeping
// unmanaged optional inputs and the identity fields the API does not return.
func databaseInputs(previous DatabaseArgs, database coolify.Database) DatabaseArgs {
	inputs := previous
	inputs.Type = DatabaseType(database.Type())
	inputs.Name = database.Name
	inputs.Description = coolify.Deref(database.Description)
	inputs.Image = ifSet(previous.Image, database.Image)
	inputs.IsPublic = database.IsPublic
	if database.IsPublic || previous.PublicPort != nil {
		inputs.PublicPort = database.PublicPort
	}
	return inputs
}

func databaseState(inputs DatabaseArgs, database coolify.Database) DatabaseState {
	user, password, name := database.Credentials()
	return DatabaseState{
		DatabaseArgs:  inputs,
		UUID:          database.UUID,
		EnvironmentID: database.EnvironmentID,
		Status:        database.Status,
		InternalURL:   coolify.Deref(database.InternalDBURL),
		ExternalURL:   coolify.Deref(database.ExternalDBURL),
		Username:      user,
		Password:      password,
		DatabaseName:  name,
	}
}
