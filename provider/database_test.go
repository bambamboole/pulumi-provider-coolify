package provider

import (
	"context"
	"strings"
	"testing"
)

func databaseArgs(projectUUID string) DatabaseArgs {
	return DatabaseArgs{
		Type:            DatabaseTypePostgreSQL,
		Name:            "app-db",
		Description:     "primary",
		ProjectUUID:     projectUUID,
		EnvironmentName: "production",
		ServerUUID:      "u-server",
	}
}

func TestCreateDatabaseResolvesEnvironmentAndKeepsDefaultImage(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production")

	database, err := createDatabase(ctx, c, databaseArgs(projectUUID))
	if err != nil {
		t.Fatalf("createDatabase: %v", err)
	}
	if database.EnvironmentID != fake.environmentID(projectUUID, "production") {
		t.Fatalf("database landed in the wrong environment: %+v", database)
	}
	if database.Image != "default:postgresql" {
		t.Fatalf("unset image must fall back to Coolify's default, got %q", database.Image)
	}
	state := databaseState(databaseArgs(projectUUID), database)
	if state.Username != "postgres" || state.Password != "generated" || state.DatabaseName != "postgres" {
		t.Fatalf("credentials not mapped: %+v", state)
	}

	// Reconciling again must adopt without patching: the empty image is unmanaged.
	patches := fake.countRequests("PATCH", "/api/v1/databases/")
	if _, err := createDatabase(ctx, c, databaseArgs(projectUUID)); err != nil {
		t.Fatalf("second createDatabase: %v", err)
	}
	if fake.countRequests("POST", "/api/v1/databases/") != 1 {
		t.Fatalf("database was recreated: %v", fake.requests)
	}
	if fake.countRequests("PATCH", "/api/v1/databases/") != patches {
		t.Fatalf("unmanaged image must not be patched: %v", fake.requests)
	}
}

func TestCreateDatabaseFailsForMissingEnvironment(t *testing.T) {
	fake := newFakeCoolify(t)
	projectUUID := fake.addProject("Main", "production")
	args := databaseArgs(projectUUID)
	args.EnvironmentName = "staging"
	_, err := createDatabase(context.Background(), fake.client(), args)
	if err == nil || !strings.Contains(err.Error(), `environment "staging" not found`) {
		t.Fatalf("expected a clear environment error, got %v", err)
	}
}

func TestCreateDatabaseRejectsAdoptingDifferentType(t *testing.T) {
	fake := newFakeCoolify(t)
	projectUUID := fake.addProject("Main", "production")
	fake.addDatabase(map[string]any{
		"name": "app-db", "database_type": "standalone-redis", "image": "redis:7",
		"environment_id": fake.environmentID(projectUUID, "production"),
	})
	_, err := createDatabase(context.Background(), fake.client(), databaseArgs(projectUUID))
	if err == nil || !strings.Contains(err.Error(), `with type "redis", expected "postgresql"`) {
		t.Fatalf("expected type mismatch error, got %v", err)
	}
}

func TestApplyDatabasePatchesOnlyChangedFields(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production")
	port := 5432
	uuid := fake.addDatabase(map[string]any{
		"name": "app-db", "description": "primary", "database_type": "standalone-postgresql", "image": "postgres:16",
		"is_public": false, "environment_id": fake.environmentID(projectUUID, "production"),
	})
	current, _ := c.GetDatabase(ctx, uuid)

	args := databaseArgs(projectUUID)
	args.Image = "postgres:17"
	args.IsPublic = true
	args.PublicPort = &port
	updated, err := applyDatabase(ctx, c, current, args)
	if err != nil {
		t.Fatalf("applyDatabase: %v", err)
	}
	if updated.Image != "postgres:17" || !updated.IsPublic || updated.PublicPort == nil || *updated.PublicPort != 5432 {
		t.Fatalf("patch not applied: %+v", updated)
	}

	patches := fake.countRequests("PATCH", "/api/v1/databases/")
	if _, err := applyDatabase(ctx, c, updated, args); err != nil {
		t.Fatalf("idempotent applyDatabase: %v", err)
	}
	if fake.countRequests("PATCH", "/api/v1/databases/") != patches {
		t.Fatalf("no-op apply must not patch: %v", fake.requests)
	}
}

func TestDatabaseInputsKeepUnmanagedAndIdentity(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	projectUUID := fake.addProject("Main", "production")
	uuid := fake.addDatabase(map[string]any{
		"name": "renamed", "description": "d", "database_type": "standalone-postgresql", "image": "postgres:16",
		"is_public": false, "environment_id": fake.environmentID(projectUUID, "production"),
	})
	database, _ := c.GetDatabase(context.Background(), uuid)

	inputs := databaseInputs(databaseArgs(projectUUID), database)
	if inputs.Name != "renamed" || inputs.Description != "d" {
		t.Fatalf("managed fields must follow Coolify: %+v", inputs)
	}
	if inputs.Image != "" {
		t.Fatalf("unset image must stay unmanaged, got %q", inputs.Image)
	}
	if inputs.ProjectUUID != projectUUID || inputs.EnvironmentName != "production" || inputs.ServerUUID != "u-server" {
		t.Fatalf("identity must be preserved: %+v", inputs)
	}
	managed := databaseArgs(projectUUID)
	managed.Image = "postgres:15"
	if inputs := databaseInputs(managed, database); inputs.Image != "postgres:16" {
		t.Fatalf("managed image must reflect drift, got %q", inputs.Image)
	}
}
