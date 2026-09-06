package provider

import (
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func previewProvider(t *testing.T) integration.Server {
	t.Helper()
	provider, err := New()
	if err != nil {
		t.Fatal(err)
	}
	server, err := integration.NewServer(t.Context(), "coolify", semver.MustParse("0.7.0"), integration.WithProvider(provider))
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func TestConfigDiffTagChangesDoNotReplaceProvider(t *testing.T) {
	legacy := property.NewMap(map[string]property.Value{
		"baseUrl":                    property.New("https://coolify.example.com"),
		"apiToken":                   property.New("test-token").WithSecret(true),
		"__pulumi-go-provider-infer": property.New(true),
	})
	current := legacy.Set("disableDefaultTags", property.New(false))
	for _, tt := range []struct {
		name      string
		old, next property.Map
		field     string
		kind      p.DiffKind
	}{
		{name: "upgrade from 0.6.0", old: legacy, next: legacy, field: "disableDefaultTags", kind: p.Add},
		{name: "unchanged", old: current, next: current},
		{name: "custom defaults", old: current, next: current.Set("defaultTags", property.New([]property.Value{property.New("managed")})), field: "defaultTags", kind: p.Add},
		{name: "change default tag", old: current.Set("defaultTags", property.New([]property.Value{property.New("pulumi")})), next: current.Set("defaultTags", property.New([]property.Value{property.New("managed")})), field: "defaultTags[0]", kind: p.Update},
		{name: "remove custom defaults", old: current.Set("defaultTags", property.New([]property.Value{property.New("managed")})), next: current, field: "defaultTags", kind: p.Delete},
		{name: "disable defaults", old: current, next: current.Set("disableDefaultTags", property.New(true)), field: "disableDefaultTags", kind: p.Update},
		{name: "enable defaults", old: current.Set("disableDefaultTags", property.New(true)), next: current, field: "disableDefaultTags", kind: p.Update},
		{name: "different instance still replaces", old: current, next: current.Set("baseUrl", property.New("https://other.example.com")), field: "baseUrl", kind: p.UpdateReplace},
		{name: "different credentials preserve replacement behavior", old: current, next: current.Set("apiToken", property.New("other-token").WithSecret(true)), field: "apiToken", kind: p.UpdateReplace},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := previewProvider(t)
			checked, err := server.CheckConfig(p.CheckRequest{State: tt.old, Inputs: tt.next})
			if err != nil || len(checked.Failures) != 0 {
				t.Fatalf("CheckConfig: %v %v", checked.Failures, err)
			}
			diff, err := server.DiffConfig(p.DiffRequest{State: tt.old, OldInputs: tt.old, Inputs: checked.Inputs})
			if err != nil {
				t.Fatal(err)
			}
			if tt.field == "" {
				if diff.HasChanges || len(diff.DetailedDiff) != 0 {
					t.Fatalf("upgrade must not replace an unchanged provider: %+v", diff)
				}
			} else if !diff.HasChanges || len(diff.DetailedDiff) != 1 || diff.DetailedDiff[tt.field].Kind != tt.kind {
				t.Fatalf("expected only %s=%s, got %+v", tt.field, tt.kind, diff)
			}
		})
	}
}

func TestDatabaseBackupPreviewAcceptsUnknownS3Storage(t *testing.T) {
	for _, tt := range []struct {
		name        string
		storage     property.Value
		wantFailure bool
	}{
		{name: "unknown", storage: property.New(property.Computed)},
		{name: "known", storage: property.New("storage-uuid")},
		{name: "missing", storage: property.New(property.Null), wantFailure: true},
		{name: "empty", storage: property.New(""), wantFailure: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checked, err := previewProvider(t).Check(p.CheckRequest{
				Urn: resource.URN("urn:pulumi:test::preview::coolify:index:DatabaseBackup::backup"),
				Inputs: property.NewMap(map[string]property.Value{
					"databaseUuid":  property.New("database-uuid"),
					"frequency":     property.New("daily"),
					"saveS3":        property.New(true),
					"s3StorageUuid": tt.storage,
				}),
			})
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantFailure {
				if len(checked.Failures) != 1 || checked.Failures[0].Property != "s3StorageUuid" {
					t.Fatalf("missing S3 storage must be rejected: %v", checked.Failures)
				}
			} else if len(checked.Failures) != 0 {
				t.Fatalf("valid preview input rejected: %v", checked.Failures)
			} else if tt.storage.IsComputed() && !checked.Inputs.Get("s3StorageUuid").IsComputed() {
				t.Fatal("Check must preserve the unknown storage UUID")
			}
		})
	}
}

func TestPreviewReadsStateBeforeTagsWereSupported(t *testing.T) {
	for _, tt := range []struct {
		token  string
		inputs map[string]property.Value
	}{
		{token: "Application", inputs: map[string]property.Value{"source": property.New("docker-image"), "dockerRegistryImageName": property.New("nginx")}},
		{token: "Database", inputs: map[string]property.Value{"type": property.New("postgresql")}},
		{token: "Service", inputs: map[string]property.Value{"type": property.New("plausible")}},
	} {
		t.Run(tt.token, func(t *testing.T) {
			fake := newFakeCoolify(t)
			server := previewProvider(t)
			if err := server.Configure(p.ConfigureRequest{Args: property.NewMap(map[string]property.Value{
				"baseUrl": property.New(fake.server.URL), "apiToken": property.New("test-token"),
			})}); err != nil {
				t.Fatal(err)
			}
			urn := resource.URN("urn:pulumi:test::preview::coolify:index:" + tt.token + "::legacy")
			inputs := property.NewMap(tt.inputs).
				Set("name", property.New("legacy")).
				Set("projectUuid", property.New(fake.addProject("Main", "production"))).
				Set("environmentName", property.New("production")).
				Set("serverUuid", property.New("u-server"))
			checked, err := server.Check(p.CheckRequest{Urn: urn, Inputs: inputs})
			if err != nil || len(checked.Failures) != 0 {
				t.Fatalf("Check: %v %v", checked.Failures, err)
			}
			created, err := server.Create(p.CreateRequest{Urn: urn, Properties: checked.Inputs})
			if err != nil {
				t.Fatal(err)
			}
			// Older providers persisted neither declared nor applied tags.
			legacy := created.Properties.Delete("appliedTags").Delete("tags")
			diff, err := server.Diff(p.DiffRequest{Urn: urn, ID: created.ID, State: legacy, Inputs: checked.Inputs})
			if err != nil {
				t.Fatalf("preview must accept pre-tag state: %v", err)
			}
			if !diff.HasChanges || len(diff.DetailedDiff) != 1 || diff.DetailedDiff["tags"].Kind != p.Update {
				t.Fatalf("legacy resource should only need its default tag: %+v", diff)
			}
			if _, err := server.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: legacy, Inputs: checked.Inputs}); err != nil {
				t.Fatalf("refresh must accept pre-tag state: %v", err)
			}
		})
	}
}
