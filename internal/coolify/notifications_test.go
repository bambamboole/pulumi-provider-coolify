package coolify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNotificationSettingsRoutes(t *testing.T) {
	for _, channel := range []string{"email", "discord", "slack", "telegram", "pushover", "webhook"} {
		t.Run(channel, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodPatch} {
				t.Run(method, func(t *testing.T) {
					want := NotificationSettings{"id": float64(7), "enabled": false, "timeout": float64(0)}
					c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != method || r.URL.Path != "/api/v1/notifications/"+channel {
							t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
						}
						if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Accept") != "application/json" {
							t.Error("missing authentication or JSON accept header")
						}
						if method == http.MethodPatch {
							if r.Header.Get("Content-Type") != "application/json" {
								t.Error("missing JSON content type")
							}
							var body NotificationSettings
							if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
								t.Errorf("decode patch: %v", err)
							}
							if !reflect.DeepEqual(body, NotificationSettings{"enabled": false, "timeout": float64(0), "password": nil}) {
								t.Errorf("patch changed or added fields: %#v", body)
							}
						}
						_ = json.NewEncoder(w).Encode(want)
					}))
					var got NotificationSettings
					var err error
					if method == http.MethodGet {
						got, err = c.GetNotificationSettings(context.Background(), channel)
					} else {
						got, err = c.UpdateNotificationSettings(context.Background(), channel, NotificationSettings{"enabled": false, "timeout": 0, "password": nil})
					}
					if err != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("settings = %#v, error = %v; want %#v", got, err, want)
					}
				})
			}
		})
	}
}

func TestNotificationSettingsRejectInvalidChannel(t *testing.T) {
	var requests atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
	}))
	for _, channel := range []string{"", "Email", "../email", "sms"} {
		if _, err := c.GetNotificationSettings(context.Background(), channel); err == nil {
			t.Errorf("GET accepted channel %q", channel)
		}
		if _, err := c.UpdateNotificationSettings(context.Background(), channel, NotificationSettings{}); err == nil {
			t.Errorf("PATCH accepted channel %q", channel)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid channels made %d requests", requests.Load())
	}
}

func TestNotificationSettingsErrors(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"message":"Validation failed."}`, http.StatusUnprocessableEntity)
			}))
			var err error
			if method == http.MethodGet {
				_, err = c.GetNotificationSettings(context.Background(), "email")
			} else {
				_, err = c.UpdateNotificationSettings(context.Background(), "email", NotificationSettings{"smtp_password": "private-secret"})
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnprocessableEntity || apiErr.Method != method || apiErr.Path != "/api/v1/notifications/email" {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Contains(err.Error(), "private-secret") {
				t.Fatal("error contains request secret")
			}
		})
	}
}

func TestNotificationSettingsRetriesPatch(t *testing.T) {
	var attempts atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body NotificationSettings
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !reflect.DeepEqual(body, NotificationSettings{"smtp_enabled": false, "smtp_timeout": float64(0)}) {
			t.Errorf("patch body lost on retry: %#v, error = %v", body, err)
		}
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, `{"message":"Too many requests"}`, http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(body)
	}), WithRetryPolicy(RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}))
	_, err := c.UpdateNotificationSettings(context.Background(), "email", NotificationSettings{"smtp_enabled": false, "smtp_timeout": 0})
	if err != nil || attempts.Load() != 2 {
		t.Fatalf("attempts = %d, error = %v", attempts.Load(), err)
	}
}

func TestNotificationSettingsRejectsInvalidJSONBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
	}))
	_, err := c.UpdateNotificationSettings(context.Background(), "email", NotificationSettings{"smtp_password": json.RawMessage(`private-secret`)})
	if err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatalf("expected a JSON encoding error without the payload, got %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid JSON made %d requests", requests.Load())
	}
}

func TestNotificationSettingsMalformedResponse(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not JSON`))
	}))
	if _, err := c.GetNotificationSettings(context.Background(), "email"); err == nil {
		t.Fatal("expected response decoding error")
	}
}
