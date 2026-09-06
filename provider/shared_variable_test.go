package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

var sharedVariableCases = []struct {
	token, scope, path, id string
	owners                 map[string]property.Value
}{
	{"TeamSharedVariable", "team", "/team/envs", "team/17", nil},
	{"ProjectSharedVariable", "project", "/projects/proj/envs", "project/proj/17", map[string]property.Value{"projectUuid": property.New("proj")}},
	{"EnvironmentSharedVariable", "environment", "/projects/proj/environments/production/envs", "environment/proj/production/17", map[string]property.Value{"projectUuid": property.New("proj"), "environmentName": property.New("production")}},
	{"ServerSharedVariable", "server", "/servers/srv/envs", "server/srv/17", map[string]property.Value{"serverUuid": property.New("srv")}},
}

type sharedVariableFake struct {
	mu          sync.Mutex
	record      map[string]any
	writes      []map[string]any
	methods     []string
	hideValue   bool
	status      int
	conflict    bool
	ownerPath   string
	ownerStatus int
}

func sharedVariableServer(t *testing.T, path string, existing bool) (integration.Server, *sharedVariableFake) {
	t.Helper()
	f := &sharedVariableFake{}
	if existing {
		f.record = map[string]any{"id": 17, "key": "TOKEN", "value": "original", "is_literal": true, "is_multiline": false, "is_shown_once": false, "comment": "from UI"}
	}
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.methods = append(f.methods, r.Method)
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing token")
		}
		if f.ownerPath != "" && r.URL.Path == "/api/v1"+f.ownerPath && r.Method == "GET" {
			w.WriteHeader(f.ownerStatus)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		if r.URL.Path != "/api/v1"+path && r.URL.Path != "/api/v1"+path+"/17" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"message":"Not found."}`))
			return
		}
		if r.Method == "POST" || r.Method == "PATCH" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			f.writes = append(f.writes, body)
			if r.Method == "POST" {
				f.record = map[string]any{"id": 17, "is_literal": false, "is_multiline": false, "is_shown_once": false, "comment": nil}
			}
			for key, value := range body {
				f.record[key] = value
			}
			if r.Method == "POST" {
				if f.conflict {
					f.record["value"] = "concurrent-writer"
					w.WriteHeader(409)
				} else {
					w.WriteHeader(201)
				}
				_, _ = w.Write([]byte(`{"id":17}`))
				return
			}
		}
		if r.Method == "DELETE" {
			f.record = nil
			_, _ = w.Write([]byte(`{"message":"Environment variable deleted."}`))
			return
		}
		response := map[string]any{}
		for k, v := range f.record {
			response[k] = v
		}
		if f.hideValue {
			delete(response, "value")
		}
		if r.Method == "GET" {
			list := []map[string]any{}
			if f.record != nil {
				list = append(list, response)
			}
			// An unrelated variable must never be adopted, patched or deleted.
			list = append(list, map[string]any{"id": 99, "key": "UNRELATED"})
			if err := json.NewEncoder(w).Encode(list); err != nil {
				t.Error(err)
			}
		} else if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(httpServer.Close)
	s := previewProvider(t)
	if err := s.Configure(p.ConfigureRequest{Args: property.NewMap(map[string]property.Value{"baseUrl": property.New(httpServer.URL), "apiToken": property.New("test-token")})}); err != nil {
		t.Fatal(err)
	}
	return s, f
}

func sharedVariableCheck(t *testing.T, s integration.Server, urn resource.URN, inputs property.Map) property.Map {
	t.Helper()
	checked, err := s.Check(p.CheckRequest{Urn: urn, Inputs: inputs})
	if err != nil || len(checked.Failures) != 0 {
		t.Fatalf("Check: %v %v", checked.Failures, err)
	}
	return checked.Inputs
}

func TestSharedVariableLifecycle(t *testing.T) {
	for _, tt := range sharedVariableCases {
		for _, adopt := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/adopt=%t", tt.scope, adopt), func(t *testing.T) {
				s, f := sharedVariableServer(t, tt.path, adopt)
				urn := resource.URN("urn:pulumi:test::shared::coolify:index:" + tt.token + "::token")
				inputs := sharedVariableCheck(t, s, urn, property.NewMap(tt.owners).Set("key", property.New("TOKEN")).Set("value", property.New("desired")).Set("isLiteral", property.New(false)))
				if !inputs.Get("value").Secret() {
					t.Fatal("value input is not secret")
				}
				created, err := s.Create(p.CreateRequest{Urn: urn, Properties: inputs})
				if err != nil {
					t.Fatal(err)
				}
				if created.ID != tt.id {
					t.Fatalf("ID = %q, want %q", created.ID, tt.id)
				}
				if created.Properties.Get("reference").AsString() != "{{"+tt.scope+".TOKEN}}" {
					t.Fatal("wrong reference")
				}
				if !created.Properties.Get("value").Secret() {
					t.Fatal("value output is not secret")
				}
				if created.Properties.Get("reference").Secret() {
					t.Fatal("reference must not inherit the value secret")
				}
				f.mu.Lock()
				if f.record["value"] != "desired" || f.record["is_literal"] != false {
					t.Error("create/adopt did not reconcile")
				}
				if adopt && f.record["comment"] != "from UI" {
					t.Error("unmanaged comment overwritten")
				}
				writes := len(f.writes)
				f.mu.Unlock()
				diff, err := s.Diff(p.DiffRequest{Urn: urn, ID: created.ID, State: created.Properties, OldInputs: inputs, Inputs: inputs})
				if err != nil || diff.HasChanges {
					t.Fatalf("unexpected diff: %+v %v", diff, err)
				}
				_, err = s.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: created.Properties, OldInputs: inputs, Inputs: inputs})
				if err != nil {
					t.Fatal(err)
				}
				if len(f.writes) != writes {
					t.Fatal("unchanged update sent PATCH")
				}
				next := inputs.Set("key", property.New("RENAMED")).Set("value", property.New("").WithSecret(true)).Set("comment", property.New(""))
				diff, err = s.Diff(p.DiffRequest{Urn: urn, ID: created.ID, State: created.Properties, OldInputs: inputs, Inputs: next})
				if err != nil || diff.DetailedDiff["key"].Kind != p.Update {
					t.Fatalf("rename must update: %+v %v", diff, err)
				}
				updated, err := s.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: created.Properties, OldInputs: inputs, Inputs: next})
				if err != nil {
					t.Fatal(err)
				}
				if f.record["key"] != "RENAMED" || f.record["value"] != "" || (f.record["comment"] != "" && f.record["comment"] != nil) {
					t.Fatal("rename/clear did not persist")
				}
				f.mu.Lock()
				f.record["value"] = "ui-drift"
				f.record["is_literal"] = true
				f.mu.Unlock()
				read, err := s.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: updated.Properties, Inputs: next})
				if err != nil {
					t.Fatal(err)
				}
				if read.Inputs.Get("value").AsString() != "ui-drift" || !read.Inputs.Get("isLiteral").AsBool() {
					t.Fatal("refresh missed drift")
				}
				imported, err := s.Read(p.ReadRequest{Urn: urn, ID: created.ID})
				if err != nil || imported.ID != created.ID {
					t.Fatalf("import failed: %v", err)
				}
				for k, v := range tt.owners {
					if !reflect.DeepEqual(imported.Inputs.Get(k), v) {
						t.Errorf("import lost %s", k)
					}
				}
				if err := s.Delete(p.DeleteRequest{Urn: urn, ID: created.ID, Properties: read.Properties, OldInputs: read.Inputs}); err != nil {
					t.Fatal(err)
				}
				if err := s.Delete(p.DeleteRequest{Urn: urn, ID: created.ID, Properties: read.Properties, OldInputs: read.Inputs}); err != nil {
					t.Fatalf("repeated delete: %v", err)
				}
				gone, err := s.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: read.Properties, Inputs: read.Inputs})
				if err != nil || gone.ID != "" {
					t.Fatalf("missing variable retained: %+v %v", gone, err)
				}
			})
		}
	}
}

func TestSharedVariableHiddenValues(t *testing.T) {
	s, f := sharedVariableServer(t, "/team/envs", true)
	f.hideValue = true
	urn := resource.URN("urn:pulumi:test::shared::coolify:index:TeamSharedVariable::token")
	inputs := sharedVariableCheck(t, s, urn, property.NewMap(map[string]property.Value{"key": property.New("TOKEN"), "value": property.New("desired")}))
	created, err := s.Create(p.CreateRequest{Urn: urn, Properties: inputs})
	if err != nil {
		t.Fatal(err)
	}
	read, err := s.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: created.Properties, Inputs: inputs})
	if err != nil || read.Properties.Get("value").AsString() != "desired" {
		t.Fatalf("hidden value lost: %v", err)
	}
	stateOnly, err := s.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: created.Properties})
	if err != nil || stateOnly.Properties.Get("value").AsString() != "desired" {
		t.Fatalf("state-only refresh lost hidden value: %v", err)
	}
	_, err = s.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: read.Properties, Inputs: inputs, OldInputs: inputs})
	if err != nil || len(f.writes) != 1 {
		t.Fatalf("hidden unchanged value patched: %v", err)
	}
	next := inputs.Set("value", property.New("rotated").WithSecret(true))
	_, err = s.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: read.Properties, Inputs: next, OldInputs: inputs})
	if err != nil || f.record["value"] != "rotated" {
		t.Fatalf("rotation failed: %v", err)
	}
	imported, err := s.Read(p.ReadRequest{Urn: urn, ID: created.ID})
	if err != nil || !imported.Inputs.Get("value").IsNull() {
		t.Fatalf("hidden import invented a value: %v", err)
	}
	f.mu.Lock()
	f.hideValue = false
	f.record["value"] = nil
	f.mu.Unlock()
	read, err = s.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: created.Properties, Inputs: inputs})
	if err != nil || read.Inputs.Get("value").AsString() != "" {
		t.Fatalf("explicit null should report cleared value: %v", err)
	}
}

func TestSharedVariablePreviewAndReplacement(t *testing.T) {
	for _, tt := range sharedVariableCases {
		t.Run(tt.scope, func(t *testing.T) {
			s := previewProvider(t) // Deliberately unconfigured: preview must not call the API.
			urn := resource.URN("urn:pulumi:test::shared::coolify:index:" + tt.token + "::token")
			inputs := property.NewMap(tt.owners).Set("key", property.New("TOKEN")).Set("value", property.New(property.Computed).WithSecret(true))
			inputs = sharedVariableCheck(t, s, urn, inputs)
			if !inputs.Get("value").IsComputed() || !inputs.Get("value").Secret() {
				t.Fatal("Check lost secret unknown")
			}
			if _, err := s.Create(p.CreateRequest{Urn: urn, Properties: inputs, DryRun: true}); err != nil {
				t.Fatal(err)
			}
			known := property.NewMap(tt.owners).Set("key", property.New("TOKEN"))
			state := known.Set("variableId", property.New(float64(17))).Set("reference", property.New("{{"+tt.scope+".TOKEN}}"))
			for owner := range tt.owners {
				next := known.Set(owner, property.New("other"))
				diff, err := s.Diff(p.DiffRequest{Urn: urn, ID: tt.id, State: state, Inputs: next, OldInputs: known})
				if err != nil || diff.DetailedDiff[owner].Kind != p.UpdateReplace {
					t.Fatalf("scope change must replace: %+v %v", diff, err)
				}
				if !diff.DeleteBeforeReplace {
					t.Fatal("same-key scope replacement must delete first to handle environment aliases")
				}
			}
			unknown := inputs.Set("key", property.New(property.Computed))
			for owner := range tt.owners {
				unknown = unknown.Set(owner, property.New(property.Computed))
			}
			unknown = sharedVariableCheck(t, s, urn, unknown)
			for _, name := range []string{"key", "value"} {
				if !unknown.Get(name).IsComputed() {
					t.Errorf("unknown %s was lost", name)
				}
			}
			for owner := range tt.owners {
				if !unknown.Get(owner).IsComputed() {
					t.Errorf("unknown %s was lost", owner)
				}
			}
			preview, err := s.Create(p.CreateRequest{Urn: urn, Properties: unknown, DryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			if !preview.Properties.Get("reference").IsComputed() {
				t.Fatal("reference is known with unknown key")
			}
			update, err := s.Update(p.UpdateRequest{Urn: urn, ID: tt.id, State: state, Inputs: unknown, OldInputs: known, DryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			if !update.Properties.Get("value").IsComputed() || !update.Properties.Get("value").Secret() {
				t.Fatal("preview update lost secret unknown")
			}
		})
	}
}

func TestSharedVariableConcurrentCreate(t *testing.T) {
	for _, tt := range sharedVariableCases {
		t.Run(tt.scope, func(t *testing.T) {
			s, f := sharedVariableServer(t, tt.path, false)
			f.conflict = true
			urn := resource.URN("urn:pulumi:test::shared::coolify:index:" + tt.token + "::token")
			inputs := sharedVariableCheck(t, s, urn, property.NewMap(tt.owners).Set("key", property.New("TOKEN")).Set("value", property.New("desired")))
			created, err := s.Create(p.CreateRequest{Urn: urn, Properties: inputs})
			if err != nil || created.ID != tt.id {
				t.Fatalf("concurrent creation failed: %v", err)
			}
			if len(f.writes) != 2 || f.record["value"] != "desired" {
				t.Fatal("conflicting create did not adopt and reconcile")
			}
		})
	}
}

func TestSharedVariableMissingScope(t *testing.T) {
	for _, tt := range sharedVariableCases[1:] {
		for _, status := range []int{200, 404, 403} {
			t.Run(fmt.Sprintf("%s/owner-%d", tt.scope, status), func(t *testing.T) {
				s, f := sharedVariableServer(t, tt.path, true)
				f.status = 404
				f.ownerStatus = status
				switch tt.scope {
				case "project":
					f.ownerPath = "/projects/proj"
				case "environment":
					f.ownerPath = "/projects/proj/production"
				case "server":
					f.ownerPath = "/servers/srv"
				}
				urn := resource.URN("urn:pulumi:test::shared::coolify:index:" + tt.token + "::token")
				read, err := s.Read(p.ReadRequest{Urn: urn, ID: tt.id})
				if status == 404 {
					if err != nil || read.ID != "" {
						t.Fatalf("deleted scope not dropped: %v", err)
					}
					state := property.NewMap(tt.owners).Set("key", property.New("TOKEN")).Set("variableId", property.New(float64(17))).Set("reference", property.New("{{"+tt.scope+".TOKEN}}"))
					if err := s.Delete(p.DeleteRequest{Urn: urn, ID: tt.id, Properties: state}); err != nil {
						t.Fatal(err)
					}
				} else if err == nil {
					t.Fatal("unavailable list endpoint incorrectly dropped state")
				}
				if status == 200 && !strings.Contains(err.Error(), "v4.3.0") {
					t.Fatalf("missing version hint: %v", err)
				}
			})
		}
	}
}

func TestSharedVariableValidation(t *testing.T) {
	s := previewProvider(t)
	urn := resource.URN("urn:pulumi:test::shared::coolify:index:ProjectSharedVariable::token")
	for _, tt := range []struct {
		key, project string
		bad          bool
	}{
		{"TOKEN", "proj", false}, {"  TOKEN  ", "proj", false}, {"app.token", "proj", false},
		{"", "proj", true}, {"1TOKEN", "proj", true}, {"TOKEN-WITH-DASH", "proj", true}, {strings.Repeat("A", 256), "proj", true}, {"TOKEN", "", true},
	} {
		t.Run(tt.key+"/"+tt.project, func(t *testing.T) {
			checked, err := s.Check(p.CheckRequest{Urn: urn, Inputs: property.NewMap(map[string]property.Value{"key": property.New(tt.key), "projectUuid": property.New(tt.project)})})
			if err != nil || (len(checked.Failures) > 0) != tt.bad {
				t.Fatalf("validation = %v %v", checked.Failures, err)
			}
			if !tt.bad && checked.Inputs.Get("key").AsString() != strings.TrimSpace(tt.key) {
				t.Fatal("key not normalized")
			}
		})
	}
}

func TestSharedVariableUnsupportedAndInvalidIdentity(t *testing.T) {
	s, f := sharedVariableServer(t, "/team/envs", true)
	urn := resource.URN("urn:pulumi:test::shared::coolify:index:TeamSharedVariable::token")
	f.status = 404
	_, err := s.Read(p.ReadRequest{Urn: urn, ID: "team/17"})
	if err == nil || !strings.Contains(err.Error(), "v4.3.0") {
		t.Fatalf("unsupported endpoint should fail with version hint: %v", err)
	}
	for _, id := range []string{"17", "team/0", "team/-1", "team/no", "project/proj/17"} {
		before := len(f.methods)
		_, err := s.Read(p.ReadRequest{Urn: urn, ID: id})
		if err == nil || len(f.methods) != before {
			t.Fatalf("invalid import ID %q accepted or sent to API: %v", id, err)
		}
	}
}
