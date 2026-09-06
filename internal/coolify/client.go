// Package coolify wraps the generated Coolify API client with authentication,
// consistent error handling, retries and hand-written models for the few
// endpoints the OpenAPI specification does not describe precisely (databases
// and S3 storages).
package coolify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

const apiPath = "/api/v1"

// Client talks to a Coolify v4 instance.
type Client struct {
	baseURL string
	api     *api.Client
}

// Option customizes a Client.
type Option func(*options)

type options struct {
	httpClient *http.Client
	retry      RetryPolicy
}

// WithHTTPClient replaces the underlying HTTP client. Its transport is wrapped
// with the retry policy.
func WithHTTPClient(hc *http.Client) Option {
	return func(o *options) { o.httpClient = hc }
}

// WithRetryPolicy overrides the retry policy. A MaxAttempts of 1 disables retries.
func WithRetryPolicy(p RetryPolicy) Option {
	return func(o *options) { o.retry = p }
}

// New returns a client for the Coolify instance at baseURL (scheme and host,
// without the /api/v1 path) authenticated with the given API token.
func New(baseURL, token string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("coolify: base URL must not be empty")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("coolify: API token must not be empty")
	}

	o := options{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		retry:      DefaultRetryPolicy,
	}
	for _, opt := range opts {
		opt(&o)
	}
	hc := *o.httpClient
	hc.Transport = newRetryTransport(hc.Transport, o.retry)

	generated, err := api.NewClient(baseURL+apiPath,
		api.WithHTTPClient(&hc),
		api.WithRequestEditorFn(bearerAuth(token)),
	)
	if err != nil {
		return nil, fmt.Errorf("coolify: %w", err)
	}
	return &Client{baseURL: baseURL, api: generated}, nil
}

// BaseURL returns the normalized base URL of the Coolify instance.
func (c *Client) BaseURL() string { return c.baseURL }

func bearerAuth(token string) api.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		return nil
	}
}

// APIError is returned for non-2xx responses from the Coolify API.
type APIError struct {
	Status int
	Method string
	Path   string
	Body   string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("coolify API: %s %s returned %d", e.Method, e.Path, e.Status)
	if m := e.Message(); m != "" {
		msg += ": " + m
	}
	return msg
}

// Message returns the "message" field Coolify puts in error bodies, or the raw
// body when it is not JSON.
func (e *APIError) Message() string {
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(e.Body), &body); err == nil && body.Message != "" {
		return body.Message
	}
	return e.Body
}

// IsNotFound reports whether err is a 404 from the Coolify API.
func IsNotFound(err error) bool { return hasStatus(err, http.StatusNotFound) }

// IsConflict reports whether err is a 409 from the Coolify API.
func IsConflict(err error) bool { return hasStatus(err, http.StatusConflict) }

func hasStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == status
}

// readBody consumes the response and converts transport errors and non-2xx
// statuses into errors.
func readBody(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, fmt.Errorf("coolify API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("coolify API: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &APIError{
			Status: resp.StatusCode,
			Method: resp.Request.Method,
			Path:   resp.Request.URL.Path,
			Body:   strings.TrimSpace(string(body)),
		}
	}
	return body, nil
}

// decode reads the response and unmarshals a 2xx JSON body into T.
func decode[T any](resp *http.Response, err error) (T, error) {
	var out T
	body, err := readBody(resp, err)
	if err != nil {
		return out, err
	}
	if len(body) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("coolify API: decode %s %s response: %w", resp.Request.Method, resp.Request.URL.Path, err)
	}
	return out, nil
}

// check reads and discards the response, returning only the error.
func check(resp *http.Response, err error) error {
	_, err = readBody(resp, err)
	return err
}

type uuidResponse struct {
	UUID string `json:"uuid"`
}

// decodeUUID decodes the {"uuid": "..."} body Coolify returns on create.
func decodeUUID(resp *http.Response, err error) (string, error) {
	out, err := decode[uuidResponse](resp, err)
	return out.UUID, err
}
