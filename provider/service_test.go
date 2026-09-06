package provider

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

const testCompose = "services:\n  web:\n    image: nginx\n"

func serviceArgs(projectUUID string) ServiceArgs {
	return ServiceArgs{
		ProjectUUID:     projectUUID,
		EnvironmentName: "production",
		ServerUUID:      "u-server",
		Name:            "plausible",
		Type:            "plausible",
	}
}

func TestCreateServiceCreatesAndPatchesSettings(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production")

	args := serviceArgs(projectUUID)
	args.Description = "analytics"
	args.ConnectToDockerNetwork = true
	args.EnvironmentVariables = map[string]string{"BASE_URL": "https://plausible.example.com"}
	service, err := createService(ctx, c, args)
	if err != nil {
		t.Fatalf("createService: %v", err)
	}
	if *service.Name != "plausible" || *service.ServiceType != "plausible" || *service.EnvironmentId != fake.environmentID(projectUUID, "production") {
		t.Fatalf("unexpected service: %+v", service)
	}
	if !*service.ConnectToDockerNetwork || *service.Description != "analytics" {
		t.Fatal("settings were not applied through the patch")
	}
	vars, _ := c.ListServiceEnvVars(ctx, *service.Uuid)
	if len(vars) != 1 || *vars[0].Key != "BASE_URL" {
		t.Fatalf("env var not created: %+v", vars)
	}

	// Reconciling again adopts by name and must not create or patch anything.
	patches := fake.countRequests("PATCH", "/api/v1/services/")
	adopted, err := createService(ctx, c, args)
	if err != nil {
		t.Fatalf("second createService: %v", err)
	}
	if *adopted.Uuid != *service.Uuid || fake.countRequests("POST", "/api/v1/services ") != 1 {
		t.Fatalf("service was recreated: %v", fake.requests)
	}
	if fake.countRequests("PATCH", "/api/v1/services/") != patches {
		t.Fatalf("no-op adoption must not patch: %v", fake.requests)
	}
}

func TestCreateServiceRejectsAdoptingDifferentType(t *testing.T) {
	fake := newFakeCoolify(t)
	projectUUID := fake.addProject("Main", "production")
	fake.addService(map[string]any{"name": "plausible", "service_type": "umami", "environment_id": fake.environmentID(projectUUID, "production")})
	_, err := createService(context.Background(), fake.client(), serviceArgs(projectUUID))
	if err == nil || !strings.Contains(err.Error(), `with type "umami", expected "plausible"`) {
		t.Fatalf("expected type mismatch error, got %v", err)
	}
}

func TestServiceComposeIsSentEncodedAndOnlyWhenChanged(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production")

	args := serviceArgs(projectUUID)
	args.Type = ""
	args.DockerCompose = testCompose
	service, err := createService(ctx, c, args)
	if err != nil {
		t.Fatalf("createService: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(testCompose))
	if fake.services[*service.Uuid]["_compose"] != encoded {
		t.Fatalf("compose must be sent base64 encoded on create: %v", fake.services[*service.Uuid]["_compose"])
	}
	if fake.countRequests("PATCH", "/api/v1/services/") != 0 {
		t.Fatalf("create must not re-send the compose file: %v", fake.requests)
	}

	// Unchanged compose: no patch. Changed compose: patched.
	if _, err := applyService(ctx, c, service, args, &args.DockerCompose); err != nil {
		t.Fatalf("applyService: %v", err)
	}
	if fake.countRequests("PATCH", "/api/v1/services/") != 0 {
		t.Fatalf("unchanged compose must not patch: %v", fake.requests)
	}
	previous := args.DockerCompose
	args.DockerCompose = testCompose + "    restart: always\n"
	if _, err := applyService(ctx, c, service, args, &previous); err != nil {
		t.Fatalf("applyService with new compose: %v", err)
	}
	if fake.countRequests("PATCH", "/api/v1/services/") != 1 || fake.services[*service.Uuid]["_compose"] == encoded {
		t.Fatalf("changed compose must patch: %v", fake.requests)
	}
	// Adoption (unknown previous compose) always sends it.
	if _, err := applyService(ctx, c, service, args, nil); err != nil {
		t.Fatalf("applyService on adoption: %v", err)
	}
	if fake.countRequests("PATCH", "/api/v1/services/") != 2 {
		t.Fatalf("adoption must send the compose file: %v", fake.requests)
	}
}

func TestServiceInputsKeepIdentityAndCompose(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	projectUUID := fake.addProject("Main", "production")
	uuid := fake.addService(map[string]any{
		"name": "renamed", "description": "d", "service_type": "plausible", "connect_to_docker_network": true,
		"environment_id": fake.environmentID(projectUUID, "production"),
	})
	service, _ := c.GetService(context.Background(), uuid)

	previous := serviceArgs(projectUUID)
	previous.Type = ""
	previous.DockerCompose = testCompose
	inputs := serviceInputs(previous, service)
	if inputs.Name != "renamed" || inputs.Description != "d" || !inputs.ConnectToDockerNetwork {
		t.Fatalf("managed fields must follow Coolify: %+v", inputs)
	}
	if inputs.DockerCompose != testCompose || inputs.Type != "" {
		t.Fatalf("compose and unset type must be kept: %+v", inputs)
	}
	if inputs.ProjectUUID != projectUUID || inputs.EnvironmentName != "production" || inputs.ServerUUID != "u-server" {
		t.Fatalf("identity must be preserved: %+v", inputs)
	}
}
