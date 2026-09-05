package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

// statefulProjectServer is an in-memory Coolify that tracks projects and
// environments, mimicking the real /projects and /projects/{uuid}/environments
// endpoints well enough for reconcile tests.
type statefulProjectServer struct {
	mu           sync.Mutex
	projects     map[string]CoolifyProject
	environments map[string][]string // projectUUID -> names
	nextID       int
	server       *httptest.Server
}

func newStatefulProjectServer(t *testing.T) *statefulProjectServer {
	s := &statefulProjectServer{
		projects:     map[string]CoolifyProject{},
		environments: map[string][]string{},
		nextID:       1,
	}
	s.server = httptest.NewServer(s.handler())
	t.Cleanup(s.server.Close)
	return s
}

func (s *statefulProjectServer) url() string { return s.server.URL }

func (s *statefulProjectServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			out := make([]CoolifyProject, 0, len(s.projects))
			for _, p := range s.projects {
				out = append(out, p)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].UUID < out[j].UUID })
			jsonResponse(w, http.StatusOK, out)
		case http.MethodPost:
			var in struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			p := CoolifyProject{ID: s.nextID, UUID: s.makeUUID(), Name: in.Name, Description: in.Description}
			s.nextID++
			s.projects[p.UUID] = p
			s.environments[p.UUID] = nil
			jsonResponse(w, http.StatusCreated, map[string]string{"uuid": p.UUID})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/projects/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		parts := splitPath(r.URL.Path)
		// /api/v1/projects/{uuid}
		if len(parts) == 4 {
			uuid := parts[3]
			switch r.Method {
			case http.MethodGet:
				p, ok := s.projects[uuid]
				if !ok {
					http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
					return
				}
				jsonResponse(w, http.StatusOK, p)
			case http.MethodDelete:
				if _, ok := s.projects[uuid]; !ok {
					http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
					return
				}
				delete(s.projects, uuid)
				delete(s.environments, uuid)
				jsonResponse(w, http.StatusOK, map[string]string{"message": "deleted"})
			}
			return
		}
		// /api/v1/projects/{uuid}/environments
		if len(parts) == 5 && parts[4] == "environments" {
			uuid := parts[3]
			if _, ok := s.projects[uuid]; !ok {
				http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
				return
			}
			switch r.Method {
			case http.MethodGet:
				names := s.environments[uuid]
				out := make([]CoolifyEnvironment, 0, len(names))
				for i, name := range names {
					out = append(out, CoolifyEnvironment{ID: i + 1, UUID: s.makeUUID(), Name: name})
				}
				jsonResponse(w, http.StatusOK, out)
			case http.MethodPost:
				var in struct {
					Name string `json:"name"`
				}
				_ = json.NewDecoder(r.Body).Decode(&in)
				s.environments[uuid] = append(s.environments[uuid], in.Name)
				jsonResponse(w, http.StatusCreated, map[string]string{"uuid": s.makeUUID()})
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		// /api/v1/projects/{uuid}/environments/{name}
		if len(parts) == 5 && parts[4] == "environments" {
			uuid := parts[3]
			name := parts[5]
			if r.Method != http.MethodDelete {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			names := s.environments[uuid]
			kept := names[:0]
			for _, n := range names {
				if n != name {
					kept = append(kept, n)
				}
			}
			s.environments[uuid] = kept
			jsonResponse(w, http.StatusOK, map[string]string{"message": "deleted"})
			return
		}
		http.NotFound(w, r)
	})
	return mux
}

func (s *statefulProjectServer) makeUUID() string {
	return "u" + strings.Repeat("0", 4) + fmt.Sprint(s.nextID+1)
}

func TestSyncProjectCreatesAndAddsEnvironments(t *testing.T) {
	srv := newStatefulProjectServer(t)
	c := NewClient(srv.url(), "test-token")

	out, err := syncProject(context.Background(), c, ProjectArgs{
		Name:         "Artisan OS",
		Description:  "main project",
		Environments: []string{"production", "development", "production"},
	})
	if err != nil {
		t.Fatalf("syncProject: %v", err)
	}
	if out.Name != "Artisan OS" || out.Description != "main project" {
		t.Fatalf("unexpected state: %+v", out)
	}
	if len(out.Environments) != 2 || out.Environments[0] != "development" || out.Environments[1] != "production" {
		t.Fatalf("environments not sorted/deduped: %+v", out.Environments)
	}
	firstUUID := out.UUID

	// Second run must be idempotent: adopt existing, do not duplicate.
	again, err := syncProject(context.Background(), c, ProjectArgs{
		Name:         "Artisan OS",
		Description:  "main project",
		Environments: []string{"production", "development"},
	})
	if err != nil {
		t.Fatalf("second syncProject: %v", err)
	}
	if again.UUID != firstUUID {
		t.Fatalf("project was recreated on second sync: %q != %q", again.UUID, firstUUID)
	}
	srv.mu.Lock()
	envCount := len(srv.environments[firstUUID])
	srv.mu.Unlock()
	if envCount != 2 {
		t.Fatalf("second sync duplicated environments: got %d", envCount)
	}

	// Description change triggers an update, but not a replace.
	updated, err := syncProject(context.Background(), c, ProjectArgs{
		Name:         "Artisan OS",
		Description:  "renamed",
		Environments: []string{"production", "development"},
	})
	if err != nil {
		t.Fatalf("third syncProject: %v", err)
	}
	if updated.UUID != firstUUID || updated.Description != "renamed" {
		t.Fatalf("description update unexpected: %+v", updated)
	}
}

func splitPath(path string) []string {
	raw := strings.Split(strings.TrimPrefix(path, "/"), "/")
	out := raw[:0]
	for _, p := range raw {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
