package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

const (
	testProjectUUID = "u-project"
	testAppUUID     = "u-app-mattermost"
	testEnvironment = "production"
	testServerUUID  = "u-server"
)

// statefulApplicationServer is an in-memory Coolify that tracks applications
// and their environment variables, mimicking the /applications and
// /applications/{uuid}/envs endpoints well enough for reconcile tests.
type statefulApplicationServer struct {
	mu           sync.Mutex
	projects     []CoolifyProject
	environments []CoolifyEnvironment
	applications []CoolifyApplication
	envVars      map[string][]CoolifyEnvironmentVariable // appUUID -> vars
	nextID       int
	server       *httptest.Server
}

func newStatefulApplicationServer(t *testing.T) *statefulApplicationServer {
	s := &statefulApplicationServer{
		projects: []CoolifyProject{
			{ID: 1, UUID: testProjectUUID, Name: "Artisan OS", Description: ""},
		},
		environments: []CoolifyEnvironment{
			{ID: 2, UUID: "u-env-production", Name: testEnvironment},
		},
		envVars: map[string][]CoolifyEnvironmentVariable{},
		nextID:  1,
	}
	s.server = httptest.NewServer(s.handler())
	t.Cleanup(s.server.Close)
	return s
}

func (s *statefulApplicationServer) url() string { return s.server.URL }

func (s *statefulApplicationServer) makeUUID(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s-%d", prefix, s.nextID)
}

func (s *statefulApplicationServer) seedApplication(name string) CoolifyApplication {
	app := CoolifyApplication{
		ID:            s.nextID,
		UUID:          s.makeUUID("app"),
		Name:          name,
		Description:   "Artisan OS team chat",
		EnvironmentID: 2,
	}
	s.applications = append(s.applications, app)
	return app
}

func (s *statefulApplicationServer) seedEnvVar(appUUID, key, value string, isPreview bool) {
	s.envVars[appUUID] = append(s.envVars[appUUID], CoolifyEnvironmentVariable{
		UUID:      s.makeUUID("env"),
		Key:       key,
		Value:     value,
		IsPreview: isPreview,
	})
}

func (s *statefulApplicationServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jsonResponse(w, http.StatusOK, s.projects)
	})
	mux.HandleFunc("/api/v1/projects/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if parts := splitPath(r.URL.Path); len(parts) == 5 && parts[4] == "environments" {
			jsonResponse(w, http.StatusOK, s.environments)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/v1/applications", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jsonResponse(w, http.StatusOK, s.applications)
	})
	mux.HandleFunc("/api/v1/applications/", s.applicationsHandler)
	return mux
}

func (s *statefulApplicationServer) applicationsHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	parts := splitPath(r.URL.Path)
	// /api/v1/applications/dockerimage
	if len(parts) == 4 && parts[3] == "dockerimage" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var in struct {
			EnvironmentName string `json:"environment_name"`
			Name            string `json:"name"`
			Description     string `json:"description"`
			Domains         string `json:"domains"`
			PortsExposes    string `json:"ports_exposes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		app := CoolifyApplication{
			ID:            s.nextID,
			UUID:          s.makeUUID("app"),
			Name:          in.Name,
			Description:   in.Description,
			FQDN:          in.Domains,
			PortsExposes:  in.PortsExposes,
			EnvironmentID: 2,
		}
		s.applications = append(s.applications, app)
		jsonResponse(w, http.StatusCreated, map[string]string{"uuid": app.UUID})
		return
	}

	// /api/v1/applications/{uuid}
	if len(parts) == 4 {
		uuid := parts[3]
		switch r.Method {
		case http.MethodGet:
			for _, app := range s.applications {
				if app.UUID == uuid {
					jsonResponse(w, http.StatusOK, app)
					return
				}
			}
			http.Error(w, `{"message":"Application not found."}`, http.StatusNotFound)
		case http.MethodPatch:
			for i := range s.applications {
				if s.applications[i].UUID != uuid {
					continue
				}
				var in struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					FQDN        string `json:"domains"`
				}
				_ = json.NewDecoder(r.Body).Decode(&in)
				if in.Name != "" {
					s.applications[i].Name = in.Name
				}
				if in.Description != "" {
					s.applications[i].Description = in.Description
				}
				if in.FQDN != "" {
					s.applications[i].FQDN = in.FQDN
				}
				jsonResponse(w, http.StatusOK, s.applications[i])
				return
			}
			http.Error(w, `{"message":"Application not found."}`, http.StatusNotFound)
		case http.MethodDelete:
			for i := range s.applications {
				if s.applications[i].UUID == uuid {
					s.applications = append(s.applications[:i], s.applications[i+1:]...)
					delete(s.envVars, uuid)
					jsonResponse(w, http.StatusOK, map[string]string{"message": "deleted"})
					return
				}
			}
			http.Error(w, `{"message":"Application not found."}`, http.StatusNotFound)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// /api/v1/applications/{uuid}/envs
	if len(parts) == 5 && parts[4] == "envs" {
		uuid := parts[3]
		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, http.StatusOK, s.envVars[uuid])
		case http.MethodPost:
			var in struct {
				Key         string `json:"key"`
				Value       string `json:"value"`
				IsPreview   bool   `json:"is_preview"`
				IsShownOnce bool   `json:"is_shown_once"`
				IsLiteral   bool   `json:"is_literal"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if !in.IsLiteral || !in.IsShownOnce {
				http.Error(w, `{"message":"expected is_literal and is_shown_once"}`, http.StatusBadRequest)
				return
			}
			env := CoolifyEnvironmentVariable{
				UUID:      s.makeUUID("env"),
				Key:       in.Key,
				Value:     in.Value,
				IsPreview: in.IsPreview,
			}
			s.envVars[uuid] = append(s.envVars[uuid], env)
			jsonResponse(w, http.StatusCreated, map[string]string{"uuid": env.UUID})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	http.NotFound(w, r)
}

func appArgs(envVars map[string]string) ApplicationArgs {
	return ApplicationArgs{
		Project:                 "Artisan OS",
		Environment:             testEnvironment,
		ServerUUID:              testServerUUID,
		Source:                  ApplicationSourceDockerImage,
		DockerRegistryImageName: "mattermost/mattermost-team-edition",
		DockerRegistryImageTag:  "latest",
		Name:                    "Mattermost",
		EnvironmentVariables:    envVars,
	}
}

func TestSyncApplicationCreatesMissingEnvVars(t *testing.T) {
	srv := newStatefulApplicationServer(t)
	c := NewClient(srv.url(), "test-token")

	out, err := syncApplication(context.Background(), c, appArgs(map[string]string{
		"MM_SQLSETTINGS_DRIVERNAME":  "postgres",
		"MM_FILESETTINGS_DRIVERNAME": "s3",
	}))
	if err != nil {
		t.Fatalf("syncApplication: %v", err)
	}
	if out.UUID == "" {
		t.Fatal("expected an application UUID")
	}
	if out.Name != "Mattermost" || out.Project != "Artisan OS" || out.Environment != testEnvironment {
		t.Fatalf("unexpected state: %+v", out)
	}
	got := out.EnvironmentVariables
	if got["MM_SQLSETTINGS_DRIVERNAME"] != "postgres" || got["MM_FILESETTINGS_DRIVERNAME"] != "s3" {
		t.Fatalf("unexpected environment variables: %+v", got)
	}

	srv.mu.Lock()
	vars := srv.envVars[out.UUID]
	srv.mu.Unlock()
	if len(vars) != 2 {
		t.Fatalf("expected 2 env vars created, got %d: %+v", len(vars), vars)
	}
	for _, env := range vars {
		if env.IsPreview {
			t.Fatalf("env var %s must not be a preview: %+v", env.Key, env)
		}
	}
}

func TestSyncApplicationLeavesExistingEnvVarsUntouched(t *testing.T) {
	srv := newStatefulApplicationServer(t)
	app := srv.seedApplication("Mattermost")
	srv.seedEnvVar(app.UUID, "MM_SQLSETTINGS_DRIVERNAME", "postgres", false)
	c := NewClient(srv.url(), "test-token")

	out, err := syncApplication(context.Background(), c, appArgs(map[string]string{
		"MM_SQLSETTINGS_DRIVERNAME": "mysql", // different value: must not be patched
		"MM_NEW_VAR":                "value",
	}))
	if err != nil {
		t.Fatalf("syncApplication: %v", err)
	}
	if out.UUID != app.UUID {
		t.Fatalf("expected adopt of %s, got %s", app.UUID, out.UUID)
	}
	if out.EnvironmentVariables["MM_NEW_VAR"] != "value" {
		t.Fatalf("new env var missing from state: %+v", out.EnvironmentVariables)
	}

	srv.mu.Lock()
	vars := srv.envVars[app.UUID]
	srv.mu.Unlock()
	if len(vars) != 2 {
		t.Fatalf("expected 2 env vars, got %d: %+v", len(vars), vars)
	}
	for _, env := range vars {
		if env.Key == "MM_SQLSETTINGS_DRIVERNAME" && env.Value != "postgres" {
			t.Fatalf("existing env var was patched: %+v", env)
		}
	}
}

func TestSyncApplicationLeavesUndeclaredEnvVarsAlone(t *testing.T) {
	srv := newStatefulApplicationServer(t)
	app := srv.seedApplication("Mattermost")
	srv.seedEnvVar(app.UUID, "MM_UNDECLARED", "secret-value", false)
	c := NewClient(srv.url(), "test-token")

	out, err := syncApplication(context.Background(), c, appArgs(map[string]string{}))
	if err != nil {
		t.Fatalf("syncApplication: %v", err)
	}
	if out.EnvironmentVariables["MM_UNDECLARED"] != "secret-value" {
		t.Fatalf("undeclared env var not preserved: %+v", out.EnvironmentVariables)
	}

	srv.mu.Lock()
	vars := srv.envVars[app.UUID]
	srv.mu.Unlock()
	if len(vars) != 1 || vars[0].Key != "MM_UNDECLARED" || vars[0].Value != "secret-value" {
		t.Fatalf("undeclared env var was touched: %+v", vars)
	}
}

func TestSyncApplicationAdoptsByNameAndCreatesMissing(t *testing.T) {
	srv := newStatefulApplicationServer(t)
	app := srv.seedApplication("Mattermost")
	c := NewClient(srv.url(), "test-token")

	out, err := syncApplication(context.Background(), c, appArgs(map[string]string{
		"MM_NEW_VAR": "value",
	}))
	if err != nil {
		t.Fatalf("syncApplication: %v", err)
	}
	if out.UUID != app.UUID {
		t.Fatalf("expected adoption of %s, got %s", app.UUID, out.UUID)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.applications) != 1 {
		t.Fatalf("application was recreated: %+v", srv.applications)
	}
	if len(srv.envVars[app.UUID]) != 1 || srv.envVars[app.UUID][0].Key != "MM_NEW_VAR" {
		t.Fatalf("missing env var not created: %+v", srv.envVars[app.UUID])
	}
}

func TestEnvironmentVariablesNeedUpdate(t *testing.T) {
	cases := []struct {
		name string
		olds map[string]string
		news map[string]string
		want bool
	}{
		{
			name: "new key is missing",
			olds: map[string]string{"A": "1"},
			news: map[string]string{"A": "1", "B": "2"},
			want: true,
		},
		{
			name: "same keys is a no-op",
			olds: map[string]string{"A": "1", "B": "2"},
			news: map[string]string{"A": "1", "B": "2"},
			want: false,
		},
		{
			name: "value change alone is ignored",
			olds: map[string]string{"A": "1"},
			news: map[string]string{"A": "42"},
			want: false,
		},
		{
			name: "removed key does not trigger",
			olds: map[string]string{"A": "1", "B": "2"},
			news: map[string]string{"A": "1"},
			want: false,
		},
		{
			name: "no declared keys",
			olds: map[string]string{"A": "1"},
			news: nil,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := environmentVariablesNeedUpdate(tc.olds, tc.news); got != tc.want {
				t.Fatalf("environmentVariablesNeedUpdate(%v, %v) = %v, want %v", tc.olds, tc.news, got, tc.want)
			}
		})
	}
}

func TestClientApplicationEnvVars(t *testing.T) {
	var created *map[string]any
	c := testServer(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/v1/applications/u-app-1/envs": requireBearer(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				jsonResponse(w, http.StatusOK, []map[string]any{
					{"uuid": "u-env-1", "key": "MM_AA", "value": "1", "is_preview": false},
					{"uuid": "u-env-2", "key": "MM_PREVIEW", "value": "x", "is_preview": true},
				})
			case http.MethodPost:
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, "bad body", http.StatusBadRequest)
					return
				}
				created = &body
				jsonResponse(w, http.StatusCreated, map[string]string{"uuid": "u-env-3"})
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	})
	ctx := context.Background()

	vars, err := c.ListApplicationEnvVars(ctx, "u-app-1")
	if err != nil {
		t.Fatalf("ListApplicationEnvVars: %v", err)
	}
	if len(vars) != 2 || vars[0].Key != "MM_AA" || !vars[1].IsPreview {
		t.Fatalf("unexpected env vars: %+v", vars)
	}

	envUUID, err := c.CreateApplicationEnvVar(ctx, "u-app-1", CreateApplicationEnvVarInput{
		Key:         "MM_BRAND_NEW",
		Value:       "v",
		IsShownOnce: true,
	})
	if err != nil {
		t.Fatalf("CreateApplicationEnvVar: %v", err)
	}
	if envUUID != "u-env-3" {
		t.Fatalf("unexpected env uuid: %q", envUUID)
	}
	if created == nil {
		t.Fatal("create request body was not captured")
	}
	body := *created
	if body["key"] != "MM_BRAND_NEW" || body["value"] != "v" {
		t.Fatalf("unexpected create body: %+v", body)
	}
	if body["is_literal"] != true || body["is_preview"] != false || body["is_shown_once"] != true {
		t.Fatalf("unexpected env var flags: %+v", body)
	}
}
