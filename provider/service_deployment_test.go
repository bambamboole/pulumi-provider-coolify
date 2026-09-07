package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

func TestServiceDeploymentLifecycle(t *testing.T) {
	fake := newFakeCoolify(t)
	id := fake.addService(map[string]any{"name": "work", "status": "running:healthy"})
	ctx := withClient(context.Background(), fake.client())
	args := ServiceDeploymentArgs{Service: id, Triggers: []string{"image-v1", "config-v1"}}
	created, err := (ServiceDeployment{}).Create(ctx, infer.CreateRequest[ServiceDeploymentArgs]{Inputs: args})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != id || created.Output.Status != "queued" {
		t.Fatalf("request acknowledgment: %#v", created)
	}
	if len(fake.requests) != 1 || fake.requests[0] != "POST /api/v1/services/"+id+"/restart" {
		t.Fatalf("must only request native restart: %v", fake.requests)
	}
	diff, err := (ServiceDeployment{}).Diff(ctx, infer.DiffRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id, Inputs: args, State: created.Output})
	if err != nil || diff.HasChanges {
		t.Fatalf("unchanged inputs must not redeploy: %#v %v", diff, err)
	}
	if _, err := (ServiceDeployment{}).Update(ctx, infer.UpdateRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id, Inputs: args, State: created.Output}); err != nil {
		t.Fatal(err)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("unchanged update mutated remote: %v", fake.requests)
	}
	next := args
	next.Triggers = []string{"image-v1", "config-v2"}
	diff, err = (ServiceDeployment{}).Diff(ctx, infer.DiffRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id, Inputs: next, State: created.Output})
	if err != nil || !diff.HasChanges || diff.DetailedDiff["triggers"].Kind != p.Update {
		t.Fatalf("trigger diff: %#v %v", diff, err)
	}
	updated, err := (ServiceDeployment{}).Update(ctx, infer.UpdateRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id, Inputs: next, State: created.Output})
	if err != nil || updated.Output.Status != "queued" || !reflect.DeepEqual(updated.Output.Triggers, next.Triggers) {
		t.Fatalf("update acknowledgment: %#v %v", updated, err)
	}
	if len(fake.requests) != 2 {
		t.Fatalf("one additional restart expected: %v", fake.requests)
	}
	// A failed runtime health status never drives this request resource.
	fake.services[id]["status"] = "exited:unhealthy"
	read, err := (ServiceDeployment{}).Read(ctx, infer.ReadRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id, Inputs: next, State: updated.Output})
	if err != nil || read.ID != id || !reflect.DeepEqual(read.State, updated.Output) {
		t.Fatalf("health must not change request acknowledgment: %#v %v", read, err)
	}
	diff, err = (ServiceDeployment{}).Diff(ctx, infer.DiffRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id, Inputs: next, State: read.State})
	if err != nil || diff.HasChanges {
		t.Fatalf("unhealthy service must not trigger redeploy: %#v %v", diff, err)
	}
	before := len(fake.requests)
	if _, err := (ServiceDeployment{}).Delete(ctx, infer.DeleteRequest[ServiceDeploymentState]{ID: id, State: read.State}); err != nil {
		t.Fatal(err)
	}
	if len(fake.requests) != before {
		t.Fatal("delete must not stop or remove service")
	}
	other := next
	other.Service = "other-service"
	diff, err = (ServiceDeployment{}).Diff(ctx, infer.DiffRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id, Inputs: other, State: read.State})
	if err != nil || diff.DetailedDiff["service"].Kind != p.UpdateReplace {
		t.Fatalf("service must replace: %#v %v", diff, err)
	}
	delete(fake.services, id)
	missing, err := (ServiceDeployment{}).Read(ctx, infer.ReadRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id, Inputs: next, State: read.State})
	if err != nil || missing.ID != "" {
		t.Fatalf("missing service: %#v %v", missing, err)
	}
}

func TestServiceDeploymentPreviewAcceptsUnknownService(t *testing.T) {
	server := previewProvider(t)
	urn := resource.URN("urn:pulumi:test::preview::coolify:index:ServiceDeployment::work")
	inputs := property.NewMap(map[string]property.Value{
		"service":  property.New(property.Computed),
		"triggers": property.New([]property.Value{property.New(property.Computed)}),
	})
	checked, err := server.Check(p.CheckRequest{Urn: urn, Inputs: inputs})
	if err != nil || len(checked.Failures) != 0 || !checked.Inputs.Get("service").IsComputed() {
		t.Fatalf("unknown service check: %#v %v", checked, err)
	}
	// No configured client: any attempted API operation would fail.
	created, err := server.Create(p.CreateRequest{Urn: urn, Properties: checked.Inputs, DryRun: true})
	if err != nil || created.ID != "" {
		t.Fatalf("preview must not queue a restart: %#v %v", created, err)
	}
	if !created.Properties.Get("status").IsComputed() {
		t.Fatalf("preview must not claim acceptance: %#v", created.Properties)
	}
	state := ServiceDeploymentState{ServiceDeploymentArgs: ServiceDeploymentArgs{Service: "service", Triggers: []string{"v1"}}, Status: "queued"}
	_, err = (ServiceDeployment{}).Update(context.Background(), infer.UpdateRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: "service", State: state, Inputs: ServiceDeploymentArgs{Service: "service", Triggers: []string{"v2"}}, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run update must not access API: %v", err)
	}
}

func TestServiceDeploymentReadFailureDoesNotQueueRestart(t *testing.T) {
	calls := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/services/service" {
			t.Errorf("unexpected mutation: %s %s", r.Method, r.URL.Path)
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer remote.Close()
	c, err := coolify.New(remote.URL, "token", coolify.WithRetryPolicy(coolify.RetryPolicy{MaxAttempts: 1}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = (ServiceDeployment{}).Read(withClient(context.Background(), c), infer.ReadRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: "service"})
	if err == nil || !strings.Contains(err.Error(), "503") || calls != 1 {
		t.Fatalf("read must report transient error: %v calls=%d", err, calls)
	}
}

func TestServiceDeploymentImportAndFailedRequests(t *testing.T) {
	fake := newFakeCoolify(t)
	ctx := withClient(context.Background(), fake.client())
	id := fake.addService(map[string]any{"name": "work"})
	imported, err := (ServiceDeployment{}).Read(ctx, infer.ReadRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id})
	if err != nil || imported.Inputs.Service != id || imported.State.Status != "" {
		t.Fatalf("import must not invent request history: %#v %v", imported, err)
	}
	if len(fake.requests) != 1 || !strings.HasPrefix(fake.requests[0], "GET ") {
		t.Fatalf("import must only read: %v", fake.requests)
	}
	diff, err := (ServiceDeployment{}).Diff(ctx, infer.DiffRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: id, State: imported.State, Inputs: ServiceDeploymentArgs{Service: id, Triggers: []string{}}})
	if err != nil || diff.HasChanges {
		t.Fatalf("nil and empty triggers must be equivalent: %#v %v", diff, err)
	}
	missing := ServiceDeploymentArgs{Service: "missing", Triggers: []string{"v1"}}
	created, err := (ServiceDeployment{}).Create(ctx, infer.CreateRequest[ServiceDeploymentArgs]{Inputs: missing})
	if !coolify.IsNotFound(err) || created.ID != "" || created.Output.Status != "" {
		t.Fatalf("failed request must not be acknowledged: %#v %v", created, err)
	}
	updated, err := (ServiceDeployment{}).Update(ctx, infer.UpdateRequest[ServiceDeploymentArgs, ServiceDeploymentState]{ID: "missing", Inputs: missing, State: ServiceDeploymentState{ServiceDeploymentArgs: ServiceDeploymentArgs{Service: "missing"}}})
	if !coolify.IsNotFound(err) || updated.Output.Status != "" {
		t.Fatalf("failed update must not be acknowledged: %#v %v", updated, err)
	}
}
