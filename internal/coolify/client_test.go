package coolify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.Handler, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL+"/", "test-token", opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewValidatesArguments(t *testing.T) {
	if _, err := New("", "token"); err == nil {
		t.Fatal("expected error for empty base URL")
	}
	if _, err := New("https://coolify.example.com", ""); err == nil {
		t.Fatal("expected error for empty token")
	}
	c, err := New("https://coolify.example.com/", "token")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.BaseURL() != "https://coolify.example.com" {
		t.Fatalf("base URL not normalized: %q", c.BaseURL())
	}
}

func TestClientSendsBearerTokenAndDecodes(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Accept") != "application/json" {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "uuid": "u-project", "name": "Main"}})
	}))
	projects, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 || Deref(projects[0].Uuid) != "u-project" || Deref(projects[0].Name) != "Main" {
		t.Fatalf("unexpected projects: %+v", projects)
	}
}

func TestClientReturnsAPIError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Project not found."}`, http.StatusNotFound)
	}), WithRetryPolicy(RetryPolicy{MaxAttempts: 1}))
	_, err := c.GetProject(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
	want := "coolify API: GET /api/v1/projects/missing returned 404: Project not found."
	if err.Error() != want {
		t.Fatalf("unexpected error message:\n got %q\nwant %q", err.Error(), want)
	}
}

func TestClientRetriesRateLimitedRequestsWithBody(t *testing.T) {
	var attempts atomic.Int32
	var bodies []string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body["name"])
		if attempts.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, `{"message":"Too many requests"}`, http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"u-new"}`))
	}), WithRetryPolicy(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}))

	uuid, err := c.CreateProject(context.Background(), "Main", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if uuid != "u-new" || attempts.Load() != 3 {
		t.Fatalf("expected success after 3 attempts, got uuid=%q attempts=%d", uuid, attempts.Load())
	}
	for i, name := range bodies {
		if name != "Main" {
			t.Fatalf("attempt %d lost the request body: %q", i+1, name)
		}
	}
}

func TestClientGivesUpAfterMaxAttempts(t *testing.T) {
	var attempts atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "upstream down", http.StatusBadGateway)
	}), WithRetryPolicy(RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond}))
	_, err := c.ListServers(context.Background())
	if !hasStatus(err, http.StatusBadGateway) {
		t.Fatalf("expected 502 error, got %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestClientDoesNotRetryClientErrors(t *testing.T) {
	var attempts atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, `{"message":"Validation failed."}`, http.StatusUnprocessableEntity)
	}))
	if err := c.DeleteServer(context.Background(), "u-server"); err == nil {
		t.Fatal("expected error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("expected a single attempt, got %d", attempts.Load())
	}
}

func TestRetryAfterHeader(t *testing.T) {
	if got := retryAfter("3"); got != 3*time.Second {
		t.Fatalf("seconds: got %s", got)
	}
	if got := retryAfter(""); got != 0 {
		t.Fatalf("empty: got %s", got)
	}
	if got := retryAfter("garbage"); got != 0 {
		t.Fatalf("garbage: got %s", got)
	}
}

func TestDatabaseCredentials(t *testing.T) {
	pg := Database{DatabaseType: "standalone-postgresql", PostgresUser: Ptr("app"), PostgresPassword: Ptr("pw"), PostgresDB: Ptr("db")}
	if user, password, name := pg.Credentials(); user != "app" || password != "pw" || name != "db" {
		t.Fatalf("postgres credentials: %q %q %q", user, password, name)
	}
	if pg.Type() != DatabaseTypePostgreSQL {
		t.Fatalf("type: %q", pg.Type())
	}
	redis := Database{DatabaseType: "standalone-redis", RedisPassword: Ptr("secret")}
	if user, password, name := redis.Credentials(); user != "" || password != "secret" || name != "" {
		t.Fatalf("redis credentials: %q %q %q", user, password, name)
	}
}

func TestCreateDatabaseUsesTypedEndpoint(t *testing.T) {
	var path string
	var body map[string]any
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"u-db"}`))
	}))
	uuid, err := c.CreateDatabase(context.Background(), DatabaseTypeRedis, CreateDatabaseInput{
		ServerUUID: "u-server", ProjectUUID: "u-project", EnvironmentName: "production", Name: "cache",
	})
	if err != nil || uuid != "u-db" {
		t.Fatalf("CreateDatabase: uuid=%q err=%v", uuid, err)
	}
	if path != "/api/v1/databases/redis" {
		t.Fatalf("unexpected path %q", path)
	}
	if body["name"] != "cache" || body["environment_name"] != "production" || body["is_public"] != false {
		t.Fatalf("unexpected body: %+v", body)
	}
	if _, ok := body["image"]; ok {
		t.Fatalf("unset image must be omitted: %+v", body)
	}
	if _, err := c.CreateDatabase(context.Background(), DatabaseType("oracle"), CreateDatabaseInput{}); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestDeployApplicationQuery(t *testing.T) {
	var query string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"deployments":[{"message":"queued","resource_uuid":"u-app","deployment_uuid":"dep-1"}]}`))
	}))
	items, err := c.DeployApplication(context.Background(), "u-app", DeployOptions{Force: true, PullRequestID: 42, DockerTag: "v1"})
	if err != nil || len(items) != 1 || items[0].DeploymentUUID != "dep-1" {
		t.Fatalf("DeployApplication: items=%+v err=%v", items, err)
	}
	for _, want := range []string{"uuid=u-app", "force=true", "pull_request_id=42", "docker_tag=v1"} {
		if !contains(query, want) {
			t.Fatalf("query %q misses %q", query, want)
		}
	}
	if contains(query, "pr=") {
		t.Fatalf("unset pr must be omitted: %q", query)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
