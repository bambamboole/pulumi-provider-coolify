package provider

import (
	"context"
	"sort"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// client returns a Coolify API client configured from the provider config.
func client(ctx context.Context) *Client {
	cfg := infer.GetConfig[Config](ctx)
	return NewClient(cfg.BaseURL, cfg.ApiToken)
}

// desc returns the normalized description: nil becomes the empty string.
func desc(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// uniqueSorted returns the deduplicated values sorted lexicographically.
func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// uniquePreservingOrder returns the deduplicated values preserving first occurrence order.
func uniquePreservingOrder(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// contains reports whether the slice contains value.
func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
