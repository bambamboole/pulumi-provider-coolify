package coolify

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy controls how transient failures (429 and 5xx gateway errors,
// connection resets) are retried.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts including the first one.
	MaxAttempts int
	// BaseDelay is the delay before the first retry; it doubles per attempt.
	BaseDelay time.Duration
	// MaxDelay caps the computed delay. A Retry-After header overrides it.
	MaxDelay time.Duration
}

// DefaultRetryPolicy retries three times with exponential backoff.
var DefaultRetryPolicy = RetryPolicy{
	MaxAttempts: 4,
	BaseDelay:   500 * time.Millisecond,
	MaxDelay:    10 * time.Second,
}

type retryTransport struct {
	base   http.RoundTripper
	policy RetryPolicy
	sleep  func(context.Context, time.Duration) error
}

func newRetryTransport(base http.RoundTripper, policy RetryPolicy) *retryTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &retryTransport{base: base, policy: policy, sleep: sleepContext}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempts := max(t.policy.MaxAttempts, 1)
	for attempt := 1; ; attempt++ {
		resp, err := t.base.RoundTrip(req)
		if attempt >= attempts || !shouldRetry(resp, err) || !canReplay(req) {
			return resp, err
		}
		delay := t.delay(attempt, resp)
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		if err := t.sleep(req.Context(), delay); err != nil {
			return nil, err
		}
		if req, err = replay(req); err != nil {
			return nil, err
		}
	}
}

func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// canReplay reports whether the request body can be sent again.
func canReplay(req *http.Request) bool {
	return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
}

func replay(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.Body != nil && req.Body != http.NoBody {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		clone.Body = body
	}
	return clone, nil
}

func (t *retryTransport) delay(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if after := retryAfter(resp.Header.Get("Retry-After")); after > 0 {
			return after
		}
	}
	delay := t.policy.BaseDelay << (attempt - 1)
	if t.policy.MaxDelay > 0 && delay > t.policy.MaxDelay {
		delay = t.policy.MaxDelay
	}
	return delay
}

func retryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(header); err == nil {
		return time.Until(at)
	}
	return 0
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
