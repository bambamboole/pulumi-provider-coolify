package provider

import (
	"context"
	"reflect"
	"strings"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

func TestTagNormalizationAndChecks(t *testing.T) {
	got := normalizeTags([]string{" Prod ", "prod", "Team-A", ""})
	if !reflect.DeepEqual(got, []string{"prod", "team-a"}) {
		t.Fatalf("normalizeTags: %v", got)
	}
	if failures := checkTags("tags", []string{"x", "ok"}); len(failures) != 1 || !strings.Contains(failures[0].Reason, `"x"`) {
		t.Fatalf("short tags must be rejected: %+v", failures)
	}
	if tagsDiffer([]string{"B", "a"}, []string{"a", "b"}) || !tagsDiffer([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("tagsDiffer must be order- and case-insensitive")
	}
	ctx := withDefaultTags(context.Background(), "pulumi")
	if got := effectiveTags(ctx, []string{"Prod", "pulumi"}); !reflect.DeepEqual(got, []string{"prod", "pulumi"}) {
		t.Fatalf("effectiveTags: %v", got)
	}
}

func TestConfigDefaultTags(t *testing.T) {
	ctx := context.Background()
	unset, _, err := infer.DefaultCheck[Config](ctx, property.NewMap(map[string]property.Value{"baseUrl": property.New("https://c"), "apiToken": property.New("t")}))
	if err != nil {
		t.Fatalf("DefaultCheck: %v", err)
	}
	if err := unset.applyDefaultTags(); err != nil || !reflect.DeepEqual(unset.DefaultTags, []string{"pulumi"}) {
		t.Fatalf(`unset defaultTags must become ["pulumi"], got %v %v`, unset.DefaultTags, err)
	}
	// Pulumi decodes an empty list to nil, so it cannot disable defaults...
	empty, _, err := infer.DefaultCheck[Config](ctx, property.NewMap(map[string]property.Value{"defaultTags": property.New([]property.Value{})}))
	if err != nil {
		t.Fatalf("DefaultCheck: %v", err)
	}
	if err := empty.applyDefaultTags(); err != nil || !reflect.DeepEqual(empty.DefaultTags, []string{"pulumi"}) {
		t.Fatalf("an empty list behaves like unset, got %#v %v", empty.DefaultTags, err)
	}
	// ...which is what disableDefaultTags is for.
	disabled := Config{DefaultTags: []string{"pulumi"}, DisableDefaultTags: true}
	if err := disabled.applyDefaultTags(); err != nil || len(disabled.DefaultTags) != 0 {
		t.Fatalf("disableDefaultTags must clear the defaults, got %v %v", disabled.DefaultTags, err)
	}
	custom, _, _ := infer.DefaultCheck[Config](ctx, property.NewMap(map[string]property.Value{"defaultTags": property.New([]property.Value{property.New(" Managed ")})}))
	if err := custom.applyDefaultTags(); err != nil || !reflect.DeepEqual(custom.DefaultTags, []string{"managed"}) {
		t.Fatalf("custom defaultTags must be normalized, got %v %v", custom.DefaultTags, err)
	}
	invalid := Config{DefaultTags: []string{"x"}}
	if err := invalid.applyDefaultTags(); err == nil {
		t.Fatal("short default tags must be rejected")
	}
}

func TestReconcileTagsAttachesDetachesAndLeavesManualTags(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	appUUID := addApp(fake)
	owner := applicationOwner(appUUID)
	fake.attachTag(appUUID, "manual")

	applied, err := reconcileTags(ctx, c, owner, []string{"pulumi", "Prod"}, nil)
	if err != nil || !reflect.DeepEqual(applied, []string{"prod", "pulumi"}) {
		t.Fatalf("reconcileTags: %v %v", applied, err)
	}
	if got := fake.tagNames(appUUID); !reflect.DeepEqual(got, []string{"manual", "prod", "pulumi"}) {
		t.Fatalf("tags after create: %v", got)
	}
	if fake.countRequests("POST", "/api/v1/applications/"+appUUID+"/tags") != 1 {
		t.Fatalf("missing tags must be attached with one request: %v", fake.requests)
	}

	// Nothing to do: only the listing.
	before := len(fake.requests)
	if _, err := reconcileTags(ctx, c, owner, []string{"prod", "pulumi"}, applied); err != nil {
		t.Fatalf("idempotent reconcile: %v", err)
	}
	if len(fake.requests) != before+1 {
		t.Fatalf("no-op reconcile must only list: %v", fake.requests[before:])
	}

	// "prod" leaves the declaration: detached and, being orphaned, deleted
	// from the team. "manual" was never applied by the provider and stays.
	applied, err = reconcileTags(ctx, c, owner, []string{"pulumi"}, applied)
	if err != nil || !reflect.DeepEqual(applied, []string{"pulumi"}) {
		t.Fatalf("reconcileTags after removal: %v %v", applied, err)
	}
	if got := fake.tagNames(appUUID); !reflect.DeepEqual(got, []string{"manual", "pulumi"}) {
		t.Fatalf("tags after removal: %v", got)
	}
	for _, tag := range fake.tags {
		if tag["name"] == "prod" {
			t.Fatal("orphaned tag must be deleted by Coolify")
		}
	}

	// Read: a tag detached in the UI drops out of the declared tags.
	tags, err := c.ListTags(ctx, owner)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	for _, tag := range tags {
		if tag.Name == "pulumi" {
			if err := c.DetachTag(ctx, owner, tag.UUID); err != nil {
				t.Fatalf("DetachTag: %v", err)
			}
		}
	}
	declared, stillApplied, err := readTags(ctx, c, owner, []string{"pulumi", "manual"}, []string{"pulumi"})
	if err != nil || !reflect.DeepEqual(declared, []string{"manual"}) || len(stillApplied) != 0 {
		t.Fatalf("readTags: declared=%v applied=%v err=%v", declared, stillApplied, err)
	}
	if declared, _, _ := readTags(ctx, c, owner, nil, nil); declared != nil {
		t.Fatalf("no declared tags must stay nil, got %v", declared)
	}
}

func TestApplicationTagsThroughResource(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := withDefaultTags(withClient(context.Background(), c), "pulumi")
	projectUUID := fake.addProject("Main", "production")

	args := applicationArgs(projectUUID, nil)
	args.Tags = []string{"prod"}
	created, err := Application{}.Create(ctx, infer.CreateRequest[ApplicationArgs]{Name: "chat", Inputs: args})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !reflect.DeepEqual(created.Output.AppliedTags, []string{"prod", "pulumi"}) || !reflect.DeepEqual(fake.tagNames(created.ID), []string{"prod", "pulumi"}) {
		t.Fatalf("default and declared tags must be attached: %v %v", created.Output.AppliedTags, fake.tagNames(created.ID))
	}

	// Unchanged inputs: no diff. Changed provider default: diff on tags.
	diff, err := Application{}.Diff(ctx, infer.DiffRequest[ApplicationArgs, ApplicationState]{ID: created.ID, Inputs: args, State: created.Output})
	if err != nil || diff.HasChanges {
		t.Fatalf("unchanged tags must not diff: %+v %v", diff.DetailedDiff, err)
	}
	changedDefaults := withDefaultTags(ctx, "managed")
	diff, _ = Application{}.Diff(changedDefaults, infer.DiffRequest[ApplicationArgs, ApplicationState]{ID: created.ID, Inputs: args, State: created.Output})
	if diff.DetailedDiff["tags"].Kind != p.Update {
		t.Fatalf("changed provider defaults must diff on tags: %+v", diff.DetailedDiff)
	}
	updated, err := Application{}.Update(changedDefaults, infer.UpdateRequest[ApplicationArgs, ApplicationState]{ID: created.ID, Inputs: args, State: created.Output})
	if err != nil || !reflect.DeepEqual(updated.Output.AppliedTags, []string{"managed", "prod"}) || !reflect.DeepEqual(fake.tagNames(created.ID), []string{"managed", "prod"}) {
		t.Fatalf("Update must swap the default tag: %v %v %v", updated.Output.AppliedTags, fake.tagNames(created.ID), err)
	}

	// Read reflects a tag removed in the UI.
	for _, tag := range mustListTags(t, c, applicationOwner(created.ID)) {
		if tag.Name == "prod" {
			_ = c.DetachTag(ctx, applicationOwner(created.ID), tag.UUID)
		}
	}
	read, err := Application{}.Read(changedDefaults, infer.ReadRequest[ApplicationArgs, ApplicationState]{ID: created.ID, Inputs: args, State: updated.Output})
	if err != nil || len(read.Inputs.Tags) != 0 || !reflect.DeepEqual(read.State.AppliedTags, []string{"managed"}) {
		t.Fatalf("Read must drop the detached tag: tags=%v applied=%v err=%v", read.Inputs.Tags, read.State.AppliedTags, err)
	}

	// Check normalizes and validates.
	resp, err := Application{}.Check(ctx, infer.CheckRequest{Name: "chat", NewInputs: property.NewMap(map[string]property.Value{
		"source": property.New("docker-image"), "dockerRegistryImageName": property.New("img"),
		"projectUuid": property.New("u"), "environmentName": property.New("e"), "serverUuid": property.New("s"),
		"tags": property.New([]property.Value{property.New(" Prod "), property.New("x")}),
	})})
	if err != nil || len(resp.Failures) != 1 || !reflect.DeepEqual(resp.Inputs.Tags, []string{"prod", "x"}) {
		t.Fatalf("Check: failures=%+v tags=%v err=%v", resp.Failures, resp.Inputs.Tags, err)
	}
}

func mustListTags(t *testing.T, c *coolify.Client, owner coolify.Owner) []coolify.Tag {
	tags, err := c.ListTags(context.Background(), owner)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	return tags
}
