package provider

import (
	"context"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

type diffArgsFixture struct {
	Name     string            `pulumi:"name"`
	Kind     string            `pulumi:"kind"`
	Port     *int              `pulumi:"port,optional"`
	Tags     []string          `pulumi:"tags,optional"`
	Labels   map[string]string `pulumi:"labels,optional"`
	internal string
}

func TestDiffArgs(t *testing.T) {
	port := 5432
	old := diffArgsFixture{Name: "a", Kind: "x", Port: &port, internal: "ignored"}
	same := old
	same.Tags = []string{}
	same.Labels = map[string]string{}
	same.internal = "changed"
	if diff := diffArgs(old, same); len(diff) != 0 {
		t.Fatalf("nil and empty collections must not diff: %+v", diff)
	}

	other := 6543
	changed := diffArgsFixture{Name: "b", Kind: "y", Port: &other, Tags: []string{"t"}}
	diff := diffArgs(old, changed, "kind")
	if len(diff) != 4 {
		t.Fatalf("expected 4 changed properties, got %+v", diff)
	}
	if diff["kind"].Kind != p.UpdateReplace {
		t.Fatalf("kind must replace: %+v", diff["kind"])
	}
	for _, name := range []string{"name", "port", "tags"} {
		if diff[name].Kind != p.Update {
			t.Fatalf("%s must update: %+v", name, diff[name])
		}
	}

	response := diffResponse(diff, true)
	if !response.HasChanges || !response.DeleteBeforeReplace {
		t.Fatalf("replace with the same identity must delete first: %+v", response)
	}
	if response := diffResponse(diff, false); response.DeleteBeforeReplace {
		t.Fatalf("replace with a new identity must create first: %+v", response)
	}
	if response := diffResponse(map[string]p.PropertyDiff{}, true); response.HasChanges {
		t.Fatalf("empty diff must report no changes: %+v", response)
	}
}

func TestPatchHelpers(t *testing.T) {
	var name, description, region *string
	var enabled *bool
	var port *int
	var patch patch

	patch.str(&name, "", "current")
	patch.str(&region, "eu", "eu")
	if patch.changed || name != nil || region != nil {
		t.Fatal("unset or equal strings must not be sent")
	}
	patch.text(&description, "", "old")
	if !patch.changed || description == nil || *description != "" {
		t.Fatal("text must clear when desired is empty")
	}
	patch.boolean(&enabled, false, false)
	patch.integer(&port, 0, nil)
	if enabled != nil || port != nil {
		t.Fatal("equal booleans and unset integers must not be sent")
	}
	patch.boolean(&enabled, false, true)
	patch.integer(&port, 22, nil)
	if enabled == nil || *enabled || port == nil || *port != 22 {
		t.Fatal("changed boolean and integer must be sent")
	}
}

// withClient injects the fake's client so resource methods that call client(ctx)
// can run without a configured provider.
func withClient(ctx context.Context, c *coolify.Client) context.Context {
	return context.WithValue(ctx, clientKey{}, c)
}
