package provider

import (
	"context"
	"testing"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

func TestCreateProjectAdoptsByNameAndAddsEnvironments(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()

	uuid, err := createProject(ctx, c, ProjectArgs{Name: "Main", Description: "main project", Environments: []string{"production", "staging"}})
	if err != nil {
		t.Fatalf("createProject: %v", err)
	}
	environments, err := c.ListEnvironments(ctx, uuid)
	if err != nil || len(environments) != 2 {
		t.Fatalf("expected 2 environments, got %+v (%v)", environments, err)
	}

	again, err := createProject(ctx, c, ProjectArgs{Name: "Main", Description: "renamed", Environments: []string{"production"}})
	if err != nil {
		t.Fatalf("second createProject: %v", err)
	}
	if again != uuid {
		t.Fatalf("project was recreated: %q != %q", again, uuid)
	}
	if fake.countRequests("POST", "/api/v1/projects") != 3 {
		t.Fatalf("expected one project and two environment creates, got %v", fake.requests)
	}
	project, _ := c.GetProject(ctx, uuid)
	if project.Description == nil || *project.Description != "renamed" {
		t.Fatalf("adopted project was not updated: %+v", project)
	}
}

func TestProjectUpdateRenamesByUUID(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	uuid := fake.addProject("Old", "production")

	if err := c.UpdateProject(ctx, uuid, api.UpdateProjectByUuidJSONRequestBody{Name: ptr("New"), Description: ptr("")}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if err := ensureEnvironments(ctx, c, uuid, []string{"production", "staging"}); err != nil {
		t.Fatalf("ensureEnvironments: %v", err)
	}
	projects, _ := c.ListProjects(ctx)
	if len(projects) != 1 || *projects[0].Name != "New" {
		t.Fatalf("rename must not create a project: %+v", projects)
	}
	if fake.countRequests("POST", "/api/v1/projects/"+uuid+"/environments") != 1 {
		t.Fatalf("expected exactly one environment create, got %v", fake.requests)
	}
}

func ptr[T any](v T) *T { return &v }
