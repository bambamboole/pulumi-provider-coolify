package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// fakeCoolify is an in-memory Coolify API covering the endpoints the resources
// use. Records are plain maps so PATCH bodies can be merged generically.
type fakeCoolify struct {
	t            *testing.T
	mu           sync.Mutex
	server       *httptest.Server
	nextID       int
	projects     map[string]map[string]any
	environments map[string][]map[string]any
	databases    map[string]map[string]any
	applications map[string]map[string]any
	envVars      map[string][]map[string]any
	deployments  map[string]map[string]any
	tasks        map[string][]map[string]any
	githubApps   map[string]map[string]any
	requests     []string
}

func newFakeCoolify(t *testing.T) *fakeCoolify {
	f := &fakeCoolify{
		t:            t,
		projects:     map[string]map[string]any{},
		environments: map[string][]map[string]any{},
		databases:    map[string]map[string]any{},
		applications: map[string]map[string]any{},
		envVars:      map[string][]map[string]any{},
		deployments:  map[string]map[string]any{},
		tasks:        map[string][]map[string]any{},
		githubApps:   map[string]map[string]any{},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeCoolify) client() *coolify.Client {
	c, err := coolify.New(f.server.URL, "test-token", coolify.WithRetryPolicy(coolify.RetryPolicy{MaxAttempts: 1}))
	if err != nil {
		f.t.Fatalf("coolify.New: %v", err)
	}
	return c
}

func (f *fakeCoolify) id() int {
	f.nextID++
	return f.nextID
}

func (f *fakeCoolify) addProject(name string, environments ...string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	uuid := fmt.Sprintf("u-project-%d", f.id())
	f.projects[uuid] = map[string]any{"id": f.nextID, "uuid": uuid, "name": name, "description": ""}
	for _, environment := range environments {
		f.environments[uuid] = append(f.environments[uuid], map[string]any{
			"id": f.id(), "uuid": fmt.Sprintf("u-env-%d", f.nextID), "name": environment, "project_id": f.projects[uuid]["id"],
		})
	}
	return uuid
}

func (f *fakeCoolify) environmentID(projectUUID, name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, environment := range f.environments[projectUUID] {
		if environment["name"] == name {
			return environment["id"].(int)
		}
	}
	f.t.Fatalf("environment %q not found in %q", name, projectUUID)
	return 0
}

func (f *fakeCoolify) addDatabase(record map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	uuid := fmt.Sprintf("u-db-%d", f.id())
	record["uuid"] = uuid
	f.databases[uuid] = record
	return uuid
}

func (f *fakeCoolify) addApplication(record map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	uuid := fmt.Sprintf("u-app-%d", f.id())
	record["uuid"] = uuid
	record["id"] = f.nextID
	f.applications[uuid] = record
	return uuid
}

func (f *fakeCoolify) addEnvVar(appUUID, key, value string, preview bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envVars[appUUID] = append(f.envVars[appUUID], map[string]any{
		"uuid": fmt.Sprintf("u-env-var-%d", f.id()), "key": key, "value": value, "is_preview": preview,
	})
}

func (f *fakeCoolify) countRequests(method, pathPrefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, request := range f.requests {
		if strings.HasPrefix(request, method+" "+pathPrefix) {
			n++
		}
	}
	return n
}

func (f *fakeCoolify) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer test-token" {
		writeError(w, http.StatusUnauthorized, "Unauthenticated.")
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/"), "/")
	switch parts[0] {
	case "projects":
		f.handleProjects(w, r, parts[1:])
	case "databases":
		f.handleDatabases(w, r, parts[1:])
	case "applications":
		f.handleApplications(w, r, parts[1:])
	case "deploy":
		f.handleDeploy(w, r)
	case "github-apps":
		f.handleGitHubApps(w, r, parts[1:])
	case "deployments":
		if d, ok := f.deployments[parts[1]]; ok {
			writeJSON(w, http.StatusOK, d)
			return
		}
		writeError(w, http.StatusNotFound, "Deployment not found.")
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

func (f *fakeCoolify) handleProjects(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0 && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, values(f.projects))
	case len(parts) == 0 && r.Method == http.MethodPost:
		body := readJSON(r)
		uuid := fmt.Sprintf("u-project-%d", f.id())
		f.projects[uuid] = map[string]any{"id": f.nextID, "uuid": uuid, "name": body["name"], "description": body["description"]}
		writeJSON(w, http.StatusCreated, map[string]any{"uuid": uuid})
	case len(parts) == 1:
		project, ok := f.projects[parts[0]]
		if !ok {
			writeError(w, http.StatusNotFound, "Project not found.")
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, project)
		case http.MethodPatch:
			merge(project, readJSON(r))
			writeJSON(w, http.StatusOK, project)
		case http.MethodDelete:
			delete(f.projects, parts[0])
			writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
		}
	case len(parts) == 2 && parts[1] == "environments":
		if _, ok := f.projects[parts[0]]; !ok {
			writeError(w, http.StatusNotFound, "Project not found.")
			return
		}
		if r.Method == http.MethodPost {
			body := readJSON(r)
			for _, environment := range f.environments[parts[0]] {
				if environment["name"] == body["name"] {
					writeError(w, http.StatusConflict, "Environment already exists.")
					return
				}
			}
			uuid := fmt.Sprintf("u-env-%d", f.id())
			f.environments[parts[0]] = append(f.environments[parts[0]], map[string]any{"id": f.nextID, "uuid": uuid, "name": body["name"]})
			writeJSON(w, http.StatusCreated, map[string]any{"uuid": uuid})
			return
		}
		writeJSON(w, http.StatusOK, nonNil(f.environments[parts[0]]))
	case len(parts) == 2:
		// GET /projects/{uuid}/{environment_name_or_uuid}
		for _, environment := range f.environments[parts[0]] {
			if environment["name"] == parts[1] || environment["uuid"] == parts[1] {
				writeJSON(w, http.StatusOK, environment)
				return
			}
		}
		writeError(w, http.StatusNotFound, "Environment not found.")
	case len(parts) == 3 && parts[1] == "environments" && r.Method == http.MethodDelete:
		kept := []map[string]any{}
		for _, environment := range f.environments[parts[0]] {
			if environment["name"] != parts[2] {
				kept = append(kept, environment)
			}
		}
		f.environments[parts[0]] = kept
		writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

func (f *fakeCoolify) handleDatabases(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0:
		writeJSON(w, http.StatusOK, values(f.databases))
	case len(parts) == 1 && r.Method == http.MethodPost:
		body := readJSON(r)
		uuid := fmt.Sprintf("u-db-%d", f.id())
		record := map[string]any{
			"uuid": uuid, "name": body["name"], "description": body["description"],
			"database_type": "standalone-" + parts[0], "image": body["image"], "is_public": body["is_public"],
			"public_port": body["public_port"], "status": "running",
		}
		if record["image"] == nil {
			record["image"] = "default:" + parts[0]
		}
		for _, environment := range f.environments[body["project_uuid"].(string)] {
			if environment["name"] == body["environment_name"] {
				record["environment_id"] = environment["id"]
			}
		}
		if parts[0] == "postgresql" {
			record["postgres_user"], record["postgres_password"], record["postgres_db"] = "postgres", "generated", "postgres"
		}
		f.databases[uuid] = record
		writeJSON(w, http.StatusCreated, map[string]any{"uuid": uuid})
	case len(parts) == 1:
		database, ok := f.databases[parts[0]]
		if !ok {
			writeError(w, http.StatusNotFound, "Database not found.")
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, database)
		case http.MethodPatch:
			merge(database, readJSON(r))
			writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
		case http.MethodDelete:
			delete(f.databases, parts[0])
			writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
		}
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

func (f *fakeCoolify) handleApplications(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0:
		writeJSON(w, http.StatusOK, values(f.applications))
	case len(parts) == 1 && r.Method == http.MethodPost:
		body := readJSON(r)
		uuid := fmt.Sprintf("u-app-%d", f.id())
		record := map[string]any{
			"id": f.nextID, "uuid": uuid, "name": body["name"], "description": body["description"], "status": "exited",
			"git_repository": body["git_repository"], "git_branch": body["git_branch"], "build_pack": body["build_pack"],
			"docker_registry_image_name": body["docker_registry_image_name"], "ports_exposes": body["ports_exposes"],
			"fqdn": body["domains"], "settings": map[string]any{"is_auto_deploy_enabled": false},
		}
		for _, environment := range f.environments[body["project_uuid"].(string)] {
			if environment["name"] == body["environment_name"] {
				record["environment_id"] = environment["id"]
			}
		}
		f.applications[uuid] = record
		writeJSON(w, http.StatusCreated, map[string]any{"uuid": uuid})
	case len(parts) == 1:
		app, ok := f.applications[parts[0]]
		if !ok {
			writeError(w, http.StatusNotFound, "Application not found.")
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, app)
		case http.MethodPatch:
			body := readJSON(r)
			if domains, ok := body["domains"]; ok {
				body["fqdn"] = domains
				delete(body, "domains")
			}
			settings := app["settings"].(map[string]any)
			for _, key := range []string{"is_auto_deploy_enabled", "is_force_https_enabled", "is_preview_deployments_enabled"} {
				if value, ok := body[key]; ok {
					settings[key] = value
					delete(body, key)
				}
			}
			merge(app, body)
			writeJSON(w, http.StatusOK, map[string]any{"uuid": parts[0]})
		case http.MethodDelete:
			delete(f.applications, parts[0])
			writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
		}
	case len(parts) >= 2 && parts[1] == "scheduled-tasks":
		f.handleScheduledTasks(w, r, parts[0], parts[2:])
	case len(parts) == 2 && parts[1] == "envs":
		if _, ok := f.applications[parts[0]]; !ok {
			writeError(w, http.StatusNotFound, "Application not found.")
			return
		}
		if r.Method == http.MethodPost {
			body := readJSON(r)
			if body["is_literal"] != true || body["is_shown_once"] != true {
				writeError(w, http.StatusBadRequest, "expected is_literal and is_shown_once")
				return
			}
			uuid := fmt.Sprintf("u-env-var-%d", f.id())
			f.envVars[parts[0]] = append(f.envVars[parts[0]], map[string]any{
				"uuid": uuid, "key": body["key"], "value": body["value"], "is_preview": body["is_preview"],
			})
			writeJSON(w, http.StatusCreated, map[string]any{"uuid": uuid})
			return
		}
		writeJSON(w, http.StatusOK, nonNil(f.envVars[parts[0]]))
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

func (f *fakeCoolify) handleScheduledTasks(w http.ResponseWriter, r *http.Request, appUUID string, parts []string) {
	if _, ok := f.applications[appUUID]; !ok {
		writeError(w, http.StatusNotFound, "Application not found.")
		return
	}
	switch {
	case len(parts) == 0 && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, nonNil(f.tasks[appUUID]))
	case len(parts) == 0 && r.Method == http.MethodPost:
		body := readJSON(r)
		task := map[string]any{"id": f.id(), "uuid": fmt.Sprintf("u-task-%d", f.nextID), "name": body["name"], "command": body["command"],
			"frequency": body["frequency"], "container": body["container"], "timeout": body["timeout"], "enabled": body["enabled"]}
		f.tasks[appUUID] = append(f.tasks[appUUID], task)
		writeJSON(w, http.StatusCreated, task)
	case len(parts) == 1:
		for i, task := range f.tasks[appUUID] {
			if task["uuid"] != parts[0] {
				continue
			}
			switch r.Method {
			case http.MethodPatch:
				merge(task, readJSON(r))
				writeJSON(w, http.StatusOK, task)
			case http.MethodDelete:
				f.tasks[appUUID] = append(f.tasks[appUUID][:i], f.tasks[appUUID][i+1:]...)
				writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
			}
			return
		}
		writeError(w, http.StatusNotFound, "Scheduled task not found.")
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

func (f *fakeCoolify) handleGitHubApps(w http.ResponseWriter, r *http.Request, parts []string) {
	hidden := func(app map[string]any) map[string]any {
		out := map[string]any{}
		for key, value := range app {
			if key != "client_secret" && key != "webhook_secret" {
				out[key] = value
			}
		}
		return out
	}
	switch {
	case len(parts) == 0 && r.Method == http.MethodGet:
		out := []map[string]any{}
		for _, app := range f.githubApps {
			out = append(out, hidden(app))
		}
		writeJSON(w, http.StatusOK, out)
	case len(parts) == 0 && r.Method == http.MethodPost:
		body := readJSON(r)
		uuid := fmt.Sprintf("u-gh-%d", f.id())
		body["id"], body["uuid"] = f.nextID, uuid
		if body["api_url"] == nil {
			body["api_url"] = "https://api.github.com"
		}
		f.githubApps[uuid] = body
		writeJSON(w, http.StatusCreated, hidden(body))
	case len(parts) == 1:
		for uuid, app := range f.githubApps {
			if fmt.Sprint(app["id"]) != parts[0] {
				continue
			}
			switch r.Method {
			case http.MethodPatch:
				merge(app, readJSON(r))
				writeJSON(w, http.StatusOK, hidden(app))
			case http.MethodDelete:
				delete(f.githubApps, uuid)
				writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
			}
			return
		}
		writeError(w, http.StatusNotFound, "GitHub app not found")
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

func (f *fakeCoolify) handleDeploy(w http.ResponseWriter, r *http.Request) {
	appUUID := r.URL.Query().Get("uuid")
	if _, ok := f.applications[appUUID]; !ok {
		writeError(w, http.StatusNotFound, "Application not found.")
		return
	}
	uuid := fmt.Sprintf("dep-%d", f.id())
	f.deployments[uuid] = map[string]any{"deployment_uuid": uuid, "application_id": appUUID, "status": "queued", "commit": "abc123"}
	writeJSON(w, http.StatusOK, map[string]any{"deployments": []map[string]any{
		{"message": "Deployment request queued.", "resource_uuid": appUUID, "deployment_uuid": uuid},
	}})
}

func values(records map[string]map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		out = append(out, record)
	}
	return out
}

func nonNil(records []map[string]any) []map[string]any {
	if records == nil {
		return []map[string]any{}
	}
	return records
}

func merge(record, patch map[string]any) {
	for key, value := range patch {
		record[key] = value
	}
}

func readJSON(r *http.Request) map[string]any {
	body := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"message": message})
}
