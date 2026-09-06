package coolify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestSharedVariableRoutes(t *testing.T) {
	for _, tc := range []struct {
		scope SharedVariableScope
		path  string
	}{
		{SharedVariableScope{Type: "team"}, "/team/envs"},
		{SharedVariableScope{Type: "project", ProjectUUID: "project-1"}, "/projects/project-1/envs"},
		{SharedVariableScope{Type: "environment", ProjectUUID: "project-1", EnvironmentName: "staging area"}, "/projects/project-1/environments/staging area/envs"},
		{SharedVariableScope{Type: "server", ServerUUID: "server-1"}, "/servers/server-1/envs"},
	} {
		t.Run(tc.scope.Type, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete} {
				t.Run(method, func(t *testing.T) {
					path := "/api/v1" + tc.path
					if method == http.MethodPatch || method == http.MethodDelete {
						path += "/42"
					}
					c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != method || r.URL.Path != path {
							t.Errorf("request = %s %s, want %s %s", r.Method, r.URL.Path, method, path)
						}
						if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Accept") != "application/json" {
							t.Error("missing authentication or JSON accept header")
						}
						if method == http.MethodPost || method == http.MethodPatch {
							if r.Header.Get("Content-Type") != "application/json" {
								t.Error("missing JSON content type")
							}
							var got map[string]any
							if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
								t.Errorf("decode request: %v", err)
							}
							want := map[string]any{"key": "EMPTY", "value": "", "is_literal": false, "is_multiline": false, "is_shown_once": false, "comment": ""}
							if !reflect.DeepEqual(got, want) {
								t.Errorf("body = %#v, want %#v", got, want)
							}
						}
						switch method {
						case http.MethodGet:
							_, _ = w.Write([]byte(`[{"id":42,"key":"EMPTY","value":"","is_literal":false,"is_multiline":false,"is_shown_once":false,"comment":""}]`))
						case http.MethodPost:
							w.WriteHeader(http.StatusCreated)
							_, _ = w.Write([]byte(`{"id":42}`))
						case http.MethodPatch:
							_, _ = w.Write([]byte(`{"id":42,"key":"EMPTY","value":"","is_literal":false,"is_multiline":false,"is_shown_once":false,"comment":""}`))
						case http.MethodDelete:
							_, _ = w.Write([]byte(`{"message":"Environment variable deleted."}`))
						}
					}))
					input := SharedVariableInput{Key: Ptr("EMPTY"), Value: Ptr(""), IsLiteral: Ptr(false), IsMultiline: Ptr(false), IsShownOnce: Ptr(false), Comment: Ptr("")}
					want := SharedVariable{ID: 42, Key: "EMPTY", Value: Ptr(""), ValuePresent: true, Comment: Ptr("")}
					ctx := context.Background()
					switch method {
					case http.MethodGet:
						got, err := c.ListSharedVariables(ctx, tc.scope)
						if err != nil || !reflect.DeepEqual(got, []SharedVariable{want}) {
							t.Fatalf("list = %#v, error = %v", got, err)
						}
					case http.MethodPost:
						id, err := c.CreateSharedVariable(ctx, tc.scope, input)
						if err != nil || id != 42 {
							t.Fatalf("create id = %d, error = %v", id, err)
						}
					case http.MethodPatch:
						got, err := c.UpdateSharedVariable(ctx, tc.scope, 42, input)
						if err != nil || !reflect.DeepEqual(got, want) {
							t.Fatalf("update = %#v, error = %v", got, err)
						}
					case http.MethodDelete:
						if err := c.DeleteSharedVariable(ctx, tc.scope, 42); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		})
	}
}

func TestSharedVariableUpdateOmitsUnsetFields(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if !reflect.DeepEqual(body, map[string]any{"comment": "new comment"}) {
			t.Errorf("patch changed or added fields: %#v", body)
		}
		_, _ = w.Write([]byte(`{"id":42,"key":"EXISTING"}`))
	}))
	_, err := c.UpdateSharedVariable(context.Background(), SharedVariableScope{Type: "team"}, 42, SharedVariableInput{Comment: Ptr("new comment")})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSharedVariableRejectsInvalidScopeAndID(t *testing.T) {
	var requests atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	ctx := context.Background()
	for _, scope := range []SharedVariableScope{
		{}, {Type: "Team"}, {Type: "other"}, {Type: "project"}, {Type: "project", ProjectUUID: " "},
		{Type: "environment"}, {Type: "environment", ProjectUUID: "project-1"},
		{Type: "environment", EnvironmentName: "production"}, {Type: "server"},
	} {
		if _, err := c.ListSharedVariables(ctx, scope); err == nil {
			t.Errorf("list accepted invalid scope %#v", scope)
		}
		if _, err := c.CreateSharedVariable(ctx, scope, SharedVariableInput{}); err == nil {
			t.Errorf("create accepted invalid scope %#v", scope)
		}
		if _, err := c.UpdateSharedVariable(ctx, scope, 1, SharedVariableInput{}); err == nil {
			t.Errorf("update accepted invalid scope %#v", scope)
		}
		if err := c.DeleteSharedVariable(ctx, scope, 1); err == nil {
			t.Errorf("delete accepted invalid scope %#v", scope)
		}
	}
	for _, id := range []int{0, -1} {
		if _, err := c.UpdateSharedVariable(ctx, SharedVariableScope{Type: "team"}, id, SharedVariableInput{}); err == nil {
			t.Errorf("update accepted invalid id %d", id)
		}
		if err := c.DeleteSharedVariable(ctx, SharedVariableScope{Type: "team"}, id); err == nil {
			t.Errorf("delete accepted invalid id %d", id)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid inputs made %d requests", requests.Load())
	}
}

func TestSharedVariableAPIErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"message":"Request failed."}`, status)
			}))
			ctx, scope := context.Background(), SharedVariableScope{Type: "team"}
			_, listErr := c.ListSharedVariables(ctx, scope)
			_, createErr := c.CreateSharedVariable(ctx, scope, SharedVariableInput{})
			_, updateErr := c.UpdateSharedVariable(ctx, scope, 42, SharedVariableInput{})
			deleteErr := c.DeleteSharedVariable(ctx, scope, 42)
			for i, err := range []error{listErr, createErr, updateErr, deleteErr} {
				method := []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete}[i]
				path := "/api/v1/team/envs"
				if i >= 2 {
					path += "/42"
				}
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.Status != status || apiErr.Method != method || apiErr.Path != path {
					t.Errorf("unexpected %s error: %v", method, err)
				}
			}
		})
	}
}

func TestSharedVariableCreateRequiresPositiveIntegerID(t *testing.T) {
	for _, body := range []string{"", `{}`, `null`, `{"id":null}`, `{"id":0}`, `{"id":-1}`, `{"id":"42"}`, `{"id":1.5}`, `not JSON`} {
		t.Run(body, func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(body))
			}))
			id, err := c.CreateSharedVariable(context.Background(), SharedVariableScope{Type: "team"}, SharedVariableInput{Key: Ptr("KEY")})
			if err == nil || id != 0 {
				t.Fatalf("invalid create response returned id = %d, error = %v", id, err)
			}
		})
	}
}

func TestSharedVariableResponseValues(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id":1,"key":"REDACTED"},
			{"id":2,"key":"NULL","value":null},
			{"id":3,"key":"EMPTY","value":""},
			{"id":4,"key":"VISIBLE","value":" a\nb "},
			{"id":5,"key":"ONCE","is_shown_once":true}
		]`))
	}))
	got, err := c.ListSharedVariables(context.Background(), SharedVariableScope{Type: "team"})
	want := []SharedVariable{
		{ID: 1, Key: "REDACTED"},
		{ID: 2, Key: "NULL", ValuePresent: true},
		{ID: 3, Key: "EMPTY", ValuePresent: true, Value: Ptr("")},
		{ID: 4, Key: "VISIBLE", ValuePresent: true, Value: Ptr(" a\nb ")},
		{ID: 5, Key: "ONCE", IsShownOnce: true},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("values = %#v, error = %v, want %#v", got, err, want)
	}
}

func TestSharedVariableResponseFlags(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"true", true}, {"false", false}, {"1", true}, {"0", false}, {`"1"`, true}, {`"0"`, false}, {"null", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			var got SharedVariable
			err := json.Unmarshal([]byte(fmt.Sprintf(`{"is_literal":%s,"is_multiline":%s,"is_shown_once":%s}`, tc.value, tc.value, tc.value)), &got)
			if err != nil || got.IsLiteral != tc.want || got.IsMultiline != tc.want || got.IsShownOnce != tc.want {
				t.Fatalf("flags = %#v, error = %v", got, err)
			}
		})
	}
	for _, value := range []string{"2", "-1", `"false"`, `"yes"`, "{}", "[]"} {
		var got SharedVariable
		if err := json.Unmarshal([]byte(`{"is_literal":`+value+`}`), &got); err == nil {
			t.Errorf("accepted invalid flag %s", value)
		}
	}
}
