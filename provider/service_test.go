package provider

import (
	"context"
	"encoding/base64"
	"reflect"
	"strings"

	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
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

func TestServiceDomainsCreateAdoptUpdateAndRefresh(t *testing.T) {
	fake := newFakeCoolify(t)
	ctx := withDefaultTags(withClient(context.Background(), fake.client()), "pulumi")
	args := serviceArgs(fake.addProject("Main", "production"))
	args.Domains = map[string]string{"web": "https://work.example.com:3010"}
	created, err := (Service{}).Create(ctx, infer.CreateRequest[ServiceArgs]{Inputs: args})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(created.Output.Domains, args.Domains) {
		t.Fatalf("create: %#v", created.Output.Domains)
	}
	if fake.countRequests("PATCH", "/api/v1/services/") != 0 {
		t.Fatalf("domains should be sent on create: %v", fake.requests)
	}
	// An unrelated container stays unmanaged, and UI changes to a managed one
	// are visible during refresh and repaired on adoption.
	applications := fake.services[created.ID]["applications"].([]map[string]any)
	applications[0]["fqdn"] = "https://drift.example.com"
	fake.services[created.ID]["applications"] = append(applications, map[string]any{"name": "admin", "fqdn": "https://admin.example.com"})
	read, err := (Service{}).Read(ctx, infer.ReadRequest[ServiceArgs, ServiceState]{ID: created.ID, Inputs: args, State: created.Output})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read.Inputs.Domains, map[string]string{"web": "https://drift.example.com"}) {
		t.Fatalf("refresh: %#v", read.Inputs.Domains)
	}
	diff, err := (Service{}).Diff(ctx, infer.DiffRequest[ServiceArgs, ServiceState]{ID: created.ID, Inputs: args, State: read.State})
	if err != nil || !diff.HasChanges {
		t.Fatalf("expected drift: %#v %v", diff, err)
	}
	adopted, err := (Service{}).Create(ctx, infer.CreateRequest[ServiceArgs]{Inputs: args})
	if err != nil || adopted.ID != created.ID {
		t.Fatalf("adoption: %#v %v", adopted, err)
	}
	if fake.countRequests("PATCH", "/api/v1/services/") != 1 {
		t.Fatalf("adoption must reconcile domains: %v", fake.requests)
	}
	next := args
	next.Domains = map[string]string{"web": ""}
	updated, err := (Service{}).Update(ctx, infer.UpdateRequest[ServiceArgs, ServiceState]{ID: created.ID, Inputs: next, State: adopted.Output})
	if err != nil {
		t.Fatal(err)
	}
	applications = fake.services[created.ID]["applications"].([]map[string]any)
	if applications[0]["fqdn"] != nil || applications[1]["fqdn"] != "https://admin.example.com" {
		t.Fatalf("clear must affect only web: %#v", applications)
	}
	read, err = (Service{}).Read(ctx, infer.ReadRequest[ServiceArgs, ServiceState]{ID: created.ID, Inputs: next, State: updated.Output})
	if err != nil || read.Inputs.Domains["web"] != "" {
		t.Fatalf("null should refresh as empty: %#v %v", read.Inputs.Domains, err)
	}
	diff, err = (Service{}).Diff(ctx, infer.DiffRequest[ServiceArgs, ServiceState]{ID: created.ID, Inputs: next, State: read.State})
	if err != nil || diff.HasChanges {
		t.Fatalf("clear must converge: %#v %v", diff, err)
	}
	next.Domains = nil
	read, err = (Service{}).Read(ctx, infer.ReadRequest[ServiceArgs, ServiceState]{ID: created.ID, Inputs: next, State: updated.Output})
	if err != nil || read.Inputs.Domains != nil {
		t.Fatalf("omitted domains must stay unmanaged: %#v %v", read.Inputs.Domains, err)
	}
	before := len(fake.requests)
	if _, err := (Service{}).Update(ctx, infer.UpdateRequest[ServiceArgs, ServiceState]{ID: created.ID, Inputs: next, State: updated.Output}); err != nil {
		t.Fatal(err)
	}
	for _, request := range fake.requests[before:] {
		if strings.HasPrefix(request, "PATCH ") {
			t.Fatalf("unmanaged domains must not patch: %v", request)
		}
	}
}

func TestServiceDomainsRefreshMissingDetails(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	args := serviceArgs(fake.addProject("Main", "production"))
	args.Domains = map[string]string{"web": "https://work.example.com"}
	id := fake.addService(map[string]any{"name": args.Name})
	current, err := c.GetService(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got := serviceInputs(args, current).Domains; !reflect.DeepEqual(got, args.Domains) {
		t.Fatalf("omitted relationship must preserve managed domains: %#v", got)
	}
	fake.services[id]["applications"] = []map[string]any{}
	current, err = c.GetService(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got := serviceInputs(args, current).Domains; !reflect.DeepEqual(got, map[string]string{"web": ""}) {
		t.Fatalf("removed application must be detected: %#v", got)
	}
}
