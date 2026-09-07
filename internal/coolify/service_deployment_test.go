package coolify

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestRestartServiceRequestAndErrors(t *testing.T) {
	for _, status := range []int{200, 400, 401, 403, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/services/work/restart" || r.URL.RawQuery != "" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"Service restarting request queued."}`))
			}), WithRetryPolicy(RetryPolicy{MaxAttempts: 1}))
			err := c.RestartService(context.Background(), "work")
			if status == 200 && err != nil {
				t.Fatal(err)
			}
			var apiErr *APIError
			if status != 200 && (!errors.As(err, &apiErr) || apiErr.Status != status) {
				t.Fatalf("expected HTTP%d error: %v", status, err)
			}
			if calls != 1 {
				t.Fatalf("expected one request: %d", calls)
			}
		})
	}
}
