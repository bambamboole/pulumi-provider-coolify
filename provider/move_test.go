package provider

import (
	"context"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestEnsurePlacementMovesOnlyWhenEnvironmentDiffers(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production", "staging")
	otherProject := fake.addProject("Other", "production")
	appUUID := fake.addApplication(map[string]any{"name": "api", "environment_id": fake.environmentID(projectUUID, "production"), "settings": map[string]any{}})
	move := func(ctx context.Context, environmentUUID string) error {
		return c.MoveApplication(ctx, appUUID, environmentUUID)
	}

	previous := placement{ProjectUUID: projectUUID, EnvironmentName: "production"}

	// Unchanged placement: nothing happens, not even a lookup.
	moved, err := ensurePlacement(ctx, c, previous, previous, fake.environmentID(projectUUID, "production"), move)
	if err != nil || moved || len(fake.requests) != 0 {
		t.Fatalf("unchanged placement must be a no-op: moved=%v err=%v requests=%v", moved, err, fake.requests)
	}

	// Move to another project: exactly one move call, no delete or create.
	target := placement{ProjectUUID: otherProject, EnvironmentName: "production"}
	moved, err = ensurePlacement(ctx, c, previous, target, fake.environmentID(projectUUID, "production"), move)
	if err != nil || !moved {
		t.Fatalf("expected a move: moved=%v err=%v", moved, err)
	}
	if fake.countRequests("POST", "/api/v1/applications/"+appUUID+"/move") != 1 || fake.countRequests("DELETE", "/api/v1/") != 0 {
		t.Fatalf("expected exactly one move: %v", fake.requests)
	}
	if fake.applications[appUUID]["environment_id"] != fake.environmentID(otherProject, "production") {
		t.Fatalf("application not re-parented: %+v", fake.applications[appUUID])
	}

	// Stale state: the inputs changed but Coolify already has the resource
	// there, so the move is skipped instead of tripping Coolify's 400.
	moved, err = ensurePlacement(ctx, c, previous, target, fake.environmentID(otherProject, "production"), move)
	if err != nil || moved {
		t.Fatalf("resource already in target must not move: moved=%v err=%v", moved, err)
	}
	if fake.countRequests("POST", "/api/v1/applications/"+appUUID+"/move") != 1 {
		t.Fatalf("unexpected second move: %v", fake.requests)
	}

	// Missing target environment: clear error, nothing moved.
	_, err = ensurePlacement(ctx, c, previous, placement{ProjectUUID: otherProject, EnvironmentName: "staging"}, fake.environmentID(otherProject, "production"), move)
	if err == nil || !strings.Contains(err.Error(), `environment "staging" not found`) {
		t.Fatalf("expected a clear environment error, got %v", err)
	}
}

func TestEnsurePlacementExplainsMissingMoveEndpoint(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	projectUUID := fake.addProject("Main", "production")
	otherProject := fake.addProject("Other", "production")
	// An unknown application UUID makes the fake answer 404, like an old
	// Coolify without the move route.
	move := func(ctx context.Context, environmentUUID string) error {
		return c.MoveApplication(ctx, "u-missing", environmentUUID)
	}
	_, err := ensurePlacement(context.Background(), c,
		placement{ProjectUUID: projectUUID, EnvironmentName: "production"},
		placement{ProjectUUID: otherProject, EnvironmentName: "production"}, 0, move)
	if err == nil || !strings.Contains(err.Error(), "Coolify v4.2.0 or newer") {
		t.Fatalf("expected version hint, got %v", err)
	}
}

func TestMoveClientsHitTheirEndpoints(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production", "staging")
	staging := ""
	for _, environment := range fake.environments[projectUUID] {
		if environment["name"] == "staging" {
			staging = environment["uuid"].(string)
		}
	}
	production := fake.environmentID(projectUUID, "production")
	dbUUID := fake.addDatabase(map[string]any{"name": "db", "database_type": "standalone-postgresql", "environment_id": production})
	svcUUID := fake.addService(map[string]any{"name": "svc", "environment_id": production})

	if err := c.MoveDatabase(ctx, dbUUID, staging); err != nil {
		t.Fatalf("MoveDatabase: %v", err)
	}
	if err := c.MoveService(ctx, svcUUID, staging); err != nil {
		t.Fatalf("MoveService: %v", err)
	}
	stagingID := fake.environmentID(projectUUID, "staging")
	if fake.databases[dbUUID]["environment_id"] != stagingID || fake.services[svcUUID]["environment_id"] != stagingID {
		t.Fatalf("resources not moved: %+v %+v", fake.databases[dbUUID], fake.services[svcUUID])
	}
	// Coolify rejects a move into the current environment.
	if err := c.MoveDatabase(ctx, dbUUID, staging); err == nil || !strings.Contains(err.Error(), "already in this environment") {
		t.Fatalf("expected Coolify's 400, got %v", err)
	}
}

func TestPlacementChangesAreUpdatesNotReplacements(t *testing.T) {
	ctx := context.Background()
	app := applicationArgs("u-project-1", nil)
	movedApp := app
	movedApp.ProjectUUID, movedApp.EnvironmentName = "u-project-2", "staging"
	diff, err := Application{}.Diff(ctx, infer.DiffRequest[ApplicationArgs, ApplicationState]{
		ID: "u-app", Inputs: movedApp, State: ApplicationState{ApplicationArgs: app},
	})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, key := range []string{"projectUuid", "environmentName"} {
		if diff.DetailedDiff[key].Kind != p.Update {
			t.Fatalf("%s must be an in-place update: %+v", key, diff.DetailedDiff)
		}
	}
	if diff.DeleteBeforeReplace {
		t.Fatal("a move must not schedule a replacement")
	}

	db := databaseArgs("u-project-1")
	movedDB := db
	movedDB.ProjectUUID = "u-project-2"
	dbDiff, err := Database{}.Diff(ctx, infer.DiffRequest[DatabaseArgs, DatabaseState]{
		ID: "u-db", Inputs: movedDB, State: DatabaseState{DatabaseArgs: db},
	})
	if err != nil || dbDiff.DetailedDiff["projectUuid"].Kind != p.Update {
		t.Fatalf("database project change must be an update: %+v %v", dbDiff.DetailedDiff, err)
	}

	svc := serviceArgs("u-project-1")
	movedSvc := svc
	movedSvc.EnvironmentName = "staging"
	svcDiff, err := Service{}.Diff(ctx, infer.DiffRequest[ServiceArgs, ServiceState]{
		ID: "u-svc", Inputs: movedSvc, State: ServiceState{ServiceArgs: svc},
	})
	if err != nil || svcDiff.DetailedDiff["environmentName"].Kind != p.Update {
		t.Fatalf("service environment change must be an update: %+v %v", svcDiff.DetailedDiff, err)
	}
	// Server changes still replace.
	svc.ServerUUID = "u-other"
	svcDiff, _ = Service{}.Diff(ctx, infer.DiffRequest[ServiceArgs, ServiceState]{
		ID: "u-svc", Inputs: svc, State: ServiceState{ServiceArgs: serviceArgs("u-project-1")},
	})
	if svcDiff.DetailedDiff["serverUuid"].Kind != p.UpdateReplace {
		t.Fatalf("server change must replace: %+v", svcDiff.DetailedDiff)
	}
}
