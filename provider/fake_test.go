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
	services     map[string]map[string]any
	backups      map[string][]map[string]any
	// storages are keyed by owner UUID; "_persistent" marks volumes, the rest
	// are file/directory mounts.
	storages map[string][]map[string]any
	// volumeBackups are keyed by storage UUID.
	volumeBackups map[string]map[string]any
	volumeRuns    map[string]int
	requests      []string
}

func newFakeCoolify(t *testing.T) *fakeCoolify {
	f := &fakeCoolify{
		t:             t,
		projects:      map[string]map[string]any{},
		environments:  map[string][]map[string]any{},
		databases:     map[string]map[string]any{},
		applications:  map[string]map[string]any{},
		envVars:       map[string][]map[string]any{},
		deployments:   map[string]map[string]any{},
		tasks:         map[string][]map[string]any{},
		githubApps:    map[string]map[string]any{},
		services:      map[string]map[string]any{},
		backups:       map[string][]map[string]any{},
		storages:      map[string][]map[string]any{},
		volumeBackups: map[string]map[string]any{},
		volumeRuns:    map[string]int{},
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

func (f *fakeCoolify) addService(record map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	uuid := fmt.Sprintf("u-svc-%d", f.id())
	record["uuid"] = uuid
	record["id"] = f.nextID
	f.services[uuid] = record
	return uuid
}

func (f *fakeCoolify) addBackup(databaseUUID string, record map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	uuid := fmt.Sprintf("u-backup-%d", f.id())
	record["uuid"] = uuid
	f.backups[databaseUUID] = append(f.backups[databaseUUID], record)
	return uuid
}

// addStorage registers a storage on an owner. Persistent volumes carry "name",
// mounts carry "is_directory"; service storages should carry "resource_uuid".
func (f *fakeCoolify) addStorage(ownerUUID string, persistent bool, record map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	uuid := fmt.Sprintf("u-storage-%d", f.id())
	record["uuid"] = uuid
	record["_persistent"] = persistent
	f.storages[ownerUUID] = append(f.storages[ownerUUID], record)
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
		// A trailing space in pathPrefix requests an exact path match.
		if strings.HasPrefix(request+" ", method+" "+pathPrefix) {
			n++
		}
	}
	return n
}

// lastRequest returns the most recent request line including its query string.
func (f *fakeCoolify) lastRequest() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

func (f *fakeCoolify) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
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
	case "services":
		f.handleServices(w, r, parts[1:])
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
	case len(parts) == 2 && parts[1] == "move":
		f.handleMove(w, r, f.databases, parts[0], "Database")
	case len(parts) >= 2 && parts[1] == "backups":
		f.handleBackups(w, r, parts[0], parts[2:])
	case len(parts) >= 2 && parts[1] == "storages":
		f.handleStorages(w, r, f.databases, parts[0], parts[2:])
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

// handleMove mirrors Coolify's moveResourceToEnvironment: a purely
// organizational change of environment_id.
func (f *fakeCoolify) handleMove(w http.ResponseWriter, r *http.Request, records map[string]map[string]any, uuid, kind string) {
	record, ok := records[uuid]
	if !ok || r.Method != http.MethodPost {
		writeError(w, http.StatusNotFound, kind+" not found.")
		return
	}
	body := readJSON(r)
	for projectUUID, environments := range f.environments {
		for _, environment := range environments {
			if environment["uuid"] != body["environment_uuid"] {
				continue
			}
			if record["environment_id"] == environment["id"] {
				writeError(w, http.StatusBadRequest, kind+" is already in this environment.")
				return
			}
			record["environment_id"] = environment["id"]
			writeJSON(w, http.StatusOK, map[string]any{
				"message": kind + " moved successfully.", "uuid": uuid,
				"project_uuid": projectUUID, "environment_uuid": environment["uuid"],
			})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Target environment not found or not owned by your team.")
}

// handleBackups mirrors the backup configuration endpoints: the list returns
// raw records with an integer s3_storage_id, create answers with the uuid only
// and patch with a message only.
func (f *fakeCoolify) handleBackups(w http.ResponseWriter, r *http.Request, databaseUUID string, parts []string) {
	database, ok := f.databases[databaseUUID]
	if !ok {
		writeError(w, http.StatusNotFound, "Database not found.")
		return
	}
	s3ID := func(body map[string]any) {
		if uuid, ok := body["s3_storage_uuid"]; ok {
			body["s3_storage_id"] = len(fmt.Sprint(uuid))
			delete(body, "s3_storage_uuid")
		}
	}
	switch {
	case len(parts) == 0 && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, nonNil(f.backups[databaseUUID]))
	case len(parts) == 0 && r.Method == http.MethodPost:
		if database["database_type"] == "standalone-redis" {
			writeError(w, http.StatusUnprocessableEntity, "Scheduled backups are not supported for this database type.")
			return
		}
		body := readJSON(r)
		if body["save_s3"] == true && body["s3_storage_uuid"] == nil {
			writeError(w, http.StatusUnprocessableEntity, "The s3_storage_uuid field is required when save_s3 is true.")
			return
		}
		s3ID(body)
		delete(body, "backup_now")
		body["uuid"] = fmt.Sprintf("u-backup-%d", f.id())
		if body["enabled"] == nil {
			body["enabled"] = true
		}
		if body["database_backup_retention_amount_locally"] == nil {
			body["database_backup_retention_amount_locally"] = 7
		}
		f.backups[databaseUUID] = append(f.backups[databaseUUID], body)
		writeJSON(w, http.StatusCreated, map[string]any{"uuid": body["uuid"], "message": "Backup configuration created successfully."})
	case len(parts) == 1:
		for i, backup := range f.backups[databaseUUID] {
			if backup["uuid"] != parts[0] {
				continue
			}
			switch r.Method {
			case http.MethodPatch:
				body := readJSON(r)
				if body["save_s3"] == true && body["s3_storage_uuid"] == nil {
					writeError(w, http.StatusUnprocessableEntity, "The s3_storage_uuid field is required when save_s3 is true.")
					return
				}
				s3ID(body)
				delete(body, "backup_now")
				merge(backup, body)
				writeJSON(w, http.StatusOK, map[string]any{"message": "Database backup configuration updated"})
			case http.MethodDelete:
				f.backups[databaseUUID] = append(f.backups[databaseUUID][:i], f.backups[databaseUUID][i+1:]...)
				writeJSON(w, http.StatusOK, map[string]any{"message": "Backup configuration and all executions deleted."})
			}
			return
		}
		writeError(w, http.StatusNotFound, "Backup configuration not found.")
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
	case len(parts) == 2 && parts[1] == "move":
		f.handleMove(w, r, f.applications, parts[0], "Application")
	case len(parts) >= 2 && parts[1] == "storages":
		f.handleStorages(w, r, f.applications, parts[0], parts[2:])
	case len(parts) >= 2 && parts[1] == "scheduled-tasks":
		f.handleScheduledTasks(w, r, parts[0], parts[2:])
	case len(parts) == 2 && parts[1] == "envs":
		if _, ok := f.applications[parts[0]]; !ok {
			writeError(w, http.StatusNotFound, "Application not found.")
			return
		}
		f.handleEnvVars(w, r, parts[0])
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

func (f *fakeCoolify) handleEnvVars(w http.ResponseWriter, r *http.Request, ownerUUID string) {
	if r.Method == http.MethodPost {
		body := readJSON(r)
		if body["is_literal"] != true || body["is_shown_once"] != true {
			writeError(w, http.StatusBadRequest, "expected is_literal and is_shown_once")
			return
		}
		uuid := fmt.Sprintf("u-env-var-%d", f.id())
		f.envVars[ownerUUID] = append(f.envVars[ownerUUID], map[string]any{
			"uuid": uuid, "key": body["key"], "value": body["value"], "is_preview": body["is_preview"],
		})
		writeJSON(w, http.StatusCreated, map[string]any{"uuid": uuid})
		return
	}
	writeJSON(w, http.StatusOK, nonNil(f.envVars[ownerUUID]))
}

func (f *fakeCoolify) handleServices(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0 && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, values(f.services))
	case len(parts) == 0 && r.Method == http.MethodPost:
		body := readJSON(r)
		uuid := fmt.Sprintf("u-svc-%d", f.id())
		record := map[string]any{
			"id": f.nextID, "uuid": uuid, "name": body["name"], "description": body["description"],
			"service_type": body["type"], "connect_to_docker_network": false,
		}
		// Coolify hides docker_compose_raw from the API; keep it aside for assertions.
		if body["docker_compose_raw"] != nil {
			record["_compose"] = body["docker_compose_raw"]
		}
		for _, environment := range f.environments[body["project_uuid"].(string)] {
			if environment["name"] == body["environment_name"] {
				record["environment_id"] = environment["id"]
			}
		}
		f.services[uuid] = record
		writeJSON(w, http.StatusCreated, map[string]any{"uuid": uuid, "domains": []string{}})
	case len(parts) == 1:
		service, ok := f.services[parts[0]]
		if !ok {
			writeError(w, http.StatusNotFound, "Service not found.")
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, service)
		case http.MethodPatch:
			body := readJSON(r)
			if compose, ok := body["docker_compose_raw"]; ok {
				service["_compose"] = compose
				delete(body, "docker_compose_raw")
			}
			merge(service, body)
			writeJSON(w, http.StatusOK, map[string]any{"uuid": parts[0]})
		case http.MethodDelete:
			delete(f.services, parts[0])
			writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
		}
	case len(parts) == 2 && parts[1] == "move":
		f.handleMove(w, r, f.services, parts[0], "Service")
	case len(parts) >= 2 && parts[1] == "storages":
		f.handleStorages(w, r, f.services, parts[0], parts[2:])
	case len(parts) == 2 && parts[1] == "envs":
		if _, ok := f.services[parts[0]]; !ok {
			writeError(w, http.StatusNotFound, "Service not found.")
			return
		}
		f.handleEnvVars(w, r, parts[0])
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

// handleStorages mirrors the storage endpoints and the volume backup schedule
// endpoints nested below them. Storage rows are returned without the
// "_persistent" marker, split into the two lists Coolify uses.
func (f *fakeCoolify) handleStorages(w http.ResponseWriter, r *http.Request, owners map[string]map[string]any, ownerUUID string, parts []string) {
	if _, ok := owners[ownerUUID]; !ok {
		writeError(w, http.StatusNotFound, "Resource not found.")
		return
	}
	public := func(record map[string]any) map[string]any {
		out := map[string]any{}
		for key, value := range record {
			if key != "_persistent" {
				out[key] = value
			}
		}
		return out
	}
	find := func(uuid string) (int, map[string]any) {
		for i, storage := range f.storages[ownerUUID] {
			if storage["uuid"] == uuid {
				return i, storage
			}
		}
		return -1, nil
	}
	switch {
	case len(parts) == 0 && r.Method == http.MethodGet:
		persistent, files := []map[string]any{}, []map[string]any{}
		for _, storage := range f.storages[ownerUUID] {
			if storage["_persistent"] == true {
				persistent = append(persistent, public(storage))
			} else {
				files = append(files, public(storage))
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"persistent_storages": persistent, "file_storages": files})
	case len(parts) == 0 && r.Method == http.MethodPost:
		body := readJSON(r)
		uuid := fmt.Sprintf("u-storage-%d", f.id())
		record := map[string]any{"uuid": uuid, "mount_path": body["mount_path"], "is_preview_suffix_enabled": false}
		if body["type"] == "persistent" {
			if body["name"] == nil {
				writeError(w, http.StatusUnprocessableEntity, "The name field is required for persistent storages.")
				return
			}
			record["_persistent"] = true
			record["name"] = ownerUUID + "-" + body["name"].(string)
			record["host_path"] = body["host_path"]
		} else {
			record["_persistent"] = false
			record["is_directory"] = body["is_directory"] == true
			record["is_host_file"] = false
			record["fs_path"] = body["fs_path"]
		}
		f.storages[ownerUUID] = append(f.storages[ownerUUID], record)
		writeJSON(w, http.StatusCreated, map[string]any{"uuid": uuid})
	case len(parts) == 0 && r.Method == http.MethodPatch:
		body := readJSON(r)
		_, storage := find(fmt.Sprint(body["uuid"]))
		if storage == nil {
			writeError(w, http.StatusNotFound, "Storage not found.")
			return
		}
		delete(body, "uuid")
		delete(body, "type")
		merge(storage, body)
		writeJSON(w, http.StatusOK, public(storage))
	case len(parts) == 1 && r.Method == http.MethodDelete:
		i, storage := find(parts[0])
		if storage == nil {
			writeError(w, http.StatusNotFound, "Storage not found.")
			return
		}
		if _, scheduled := f.volumeBackups[parts[0]]; scheduled {
			writeError(w, http.StatusUnprocessableEntity, "Delete this volume backup schedule and its archives before deleting the volume.")
			return
		}
		f.storages[ownerUUID] = append(f.storages[ownerUUID][:i], f.storages[ownerUUID][i+1:]...)
		writeJSON(w, http.StatusOK, map[string]any{"message": "Storage deleted."})
	case len(parts) >= 2 && parts[1] == "backups":
		_, storage := find(parts[0])
		if storage == nil {
			writeError(w, http.StatusNotFound, "Storage not found.")
			return
		}
		f.handleVolumeBackup(w, r, storage, parts[2:])
	default:
		writeError(w, http.StatusNotFound, "No route.")
	}
}

// handleVolumeBackup mirrors Coolify's upsert semantics: omitted fields fall
// back to the defaults, the response echoes the full schedule.
func (f *fakeCoolify) handleVolumeBackup(w http.ResponseWriter, r *http.Request, storage map[string]any, parts []string) {
	storageUUID := storage["uuid"].(string)
	storageType := "persistent"
	if storage["_persistent"] != true {
		storageType = "directory"
	}
	switch {
	case len(parts) == 0 && r.Method == http.MethodPut:
		if storage["_persistent"] != true && storage["is_directory"] != true {
			writeError(w, http.StatusUnprocessableEntity, "Only directory file storages can be backed up.")
			return
		}
		body := readJSON(r)
		if body["save_s3"] == true && body["s3_storage_uuid"] == nil {
			writeError(w, http.StatusUnprocessableEntity, "Select a usable S3 storage owned by your team.")
			return
		}
		if body["disable_local_backup"] == true && body["save_s3"] != true {
			writeError(w, http.StatusUnprocessableEntity, "Local backups can only be disabled when S3 backups are enabled.")
			return
		}
		defaults := map[string]any{
			"enabled": true, "save_s3": false, "disable_local_backup": false, "stop_during_backup": false, "s3_storage_uuid": nil,
			"retention_amount_locally": 7, "retention_days_locally": 0, "retention_max_storage_locally": 0,
			"retention_amount_s3": 7, "retention_days_s3": 0, "retention_max_storage_s3": 0,
		}
		existing, created := f.volumeBackups[storageUUID]
		if !created {
			existing = map[string]any{"uuid": fmt.Sprintf("u-vbackup-%d", f.id()), "timeout": 3600}
			f.volumeBackups[storageUUID] = existing
		}
		merge(existing, defaults)
		merge(existing, body)
		status := http.StatusOK
		if !created {
			status = http.StatusCreated
		}
		response := map[string]any{"message": "Storage backup schedule set.", "storage_uuid": storageUUID, "storage_type": storageType}
		merge(response, existing)
		writeJSON(w, status, response)
	case len(parts) == 0 && r.Method == http.MethodDelete:
		if _, ok := f.volumeBackups[storageUUID]; !ok {
			writeError(w, http.StatusNotFound, "Storage backup schedule not found.")
			return
		}
		delete(f.volumeBackups, storageUUID)
		writeJSON(w, http.StatusOK, map[string]any{"message": "Storage backup schedule and archives deleted."})
	case len(parts) == 1 && parts[0] == "run" && r.Method == http.MethodPost:
		if _, ok := f.volumeBackups[storageUUID]; !ok {
			writeError(w, http.StatusNotFound, "Storage backup schedule not found.")
			return
		}
		f.volumeRuns[storageUUID]++
		writeJSON(w, http.StatusOK, map[string]any{"message": "Storage backup queued."})
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
