package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// testServer starts a fake Coolify API bound to the given routes and returns
// the server plus a client pointed at it.
func testServer(t *testing.T, routes map[string]func(http.ResponseWriter, *http.Request)) *Client {
	t.Helper()
	mux := http.NewServeMux()
	for pattern, handler := range routes {
		mux.HandleFunc(pattern, handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "test-token")
}

func jsonResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func requireBearer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func TestClientListProjects(t *testing.T) {
	c := testServer(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/v1/projects": requireBearer(func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, http.StatusOK, []map[string]any{
				{"id": 1, "uuid": "u-project-1", "name": "Artisan OS", "description": "main"},
			})
		}),
	})
	projects, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 || projects[0].UUID != "u-project-1" || projects[0].Name != "Artisan OS" {
		t.Fatalf("unexpected projects: %+v", projects)
	}
}

func TestClientCreateApplication(t *testing.T) {
	c := testServer(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/v1/applications/public": requireBearer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "expected POST", http.StatusMethodNotAllowed)
				return
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			if body["git_repository"] != "https://github.com/artisan-os/docs" {
				http.Error(w, `{"message":"missing git_repository"}`, http.StatusBadRequest)
				return
			}
			jsonResponse(w, http.StatusCreated, map[string]any{"uuid": "u-app-1"})
		}),
	})
	uuid, err := c.CreateApplicationBySource(context.Background(), "/applications/public", map[string]any{
		"git_repository": "https://github.com/artisan-os/docs",
	})
	if err != nil {
		t.Fatalf("CreateApplicationBySource: %v", err)
	}
	if uuid != "u-app-1" {
		t.Fatalf("unexpected uuid: %q", uuid)
	}
}

func TestClientHandlesAPIError(t *testing.T) {
	c := testServer(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/v1/projects": requireBearer(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"message":"projects not found"}`, http.StatusNotFound)
		}),
	})
	_, err := c.ListProjects(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !NotFound(err) {
		t.Fatalf("expected NotFound, got %T %v", err, err)
	}
	if apiErr, ok := err.(*APIError); !ok || apiErr.Status != http.StatusNotFound {
		t.Fatalf("unexpected API error: %+v", err)
	}
}

func TestClientBaseURLNormalization(t *testing.T) {
	c := NewClient("https://coolify.example.com/", "token")
	if c.BaseURL != "https://coolify.example.com" {
		t.Fatalf("base URL not trimmed: %q", c.BaseURL)
	}
}

func TestClientDeployApplicationQuery(t *testing.T) {
	var sawQuery string
	c := testServer(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/v1/deploy": requireBearer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "expected POST", http.StatusMethodNotAllowed)
				return
			}
			sawQuery = r.URL.RawQuery
			jsonResponse(w, http.StatusOK, map[string]any{
				"deployments": []map[string]string{{
					"message":         "Deployment request queued.",
					"resource_uuid":   "u-app-1",
					"deployment_uuid": "dep-1",
				}},
			})
		}),
	})
	items, err := c.DeployApplication(context.Background(), "u-app-1", DeployOptions{
		Force:         true,
		PullRequestID: 42,
		DockerTag:     "v1.2.3",
	})
	if err != nil {
		t.Fatalf("DeployApplication: %v", err)
	}
	if len(items) != 1 || items[0].DeploymentUUID != "dep-1" {
		t.Fatalf("unexpected queue items: %+v", items)
	}
	query, err := url.ParseQuery(sawQuery)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if query.Get("uuid") != "u-app-1" || query.Get("force") != "true" ||
		query.Get("pull_request_id") != "42" || query.Get("docker_tag") != "v1.2.3" {
		t.Fatalf("unexpected deploy query: %q", sawQuery)
	}
}
