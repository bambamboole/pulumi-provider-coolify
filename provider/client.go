// Package provider implements the native Pulumi provider for Coolify.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// APIError is returned for non-2xx responses from the Coolify API.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("coolify API error: %d: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("coolify API error: %d", e.Status)
}

// NotFound reports whether err represents a 404 from the Coolify API.
func NotFound(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.Status == http.StatusNotFound
}

// Conflict reports whether err represents a 409 from the Coolify API.
func Conflict(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.Status == http.StatusConflict
}

// Client talks to the Coolify v4 API (the /api/v1 base is appended automatically).
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient returns a client for the given Coolify instance and API token.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Do performs an authenticated request and decodes the JSON response body into
// out. A nil out skips decoding. It returns an *APIError for non-2xx statuses.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}

	url := c.BaseURL + "/api/v1" + path
	request, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.Token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return fmt.Errorf("coolify API unreachable at %s: %w", c.BaseURL, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return &APIError{Status: response.StatusCode, Body: string(responseBody)}
	}

	if out == nil || len(responseBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
