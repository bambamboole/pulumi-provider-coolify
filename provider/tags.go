package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// defaultTagsKey carries provider default tags injected directly into the
// context; tests use it because infer.GetConfig needs a configured provider.
type defaultTagsKey struct{}

// normalizeTag applies Coolify's normalization: lower case and trimmed.
func normalizeTag(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// normalizeTags returns the normalized, de-duplicated and sorted tag names.
func normalizeTags(names []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = normalizeTag(name)
		if _, ok := seen[name]; ok || name == "" {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// checkTags reports declared tag names Coolify would reject.
func checkTags(property string, names []string) []p.CheckFailure {
	var failures []p.CheckFailure
	for _, name := range names {
		if len([]rune(normalizeTag(name))) < 2 {
			failures = append(failures, p.CheckFailure{Property: property, Reason: fmt.Sprintf("tag %q must be at least 2 characters long", name)})
		}
	}
	return failures
}

// providerDefaultTags returns the provider's default tags.
func providerDefaultTags(ctx context.Context) []string {
	if tags, ok := ctx.Value(defaultTagsKey{}).([]string); ok {
		return tags
	}
	return infer.GetConfig[Config](ctx).DefaultTags
}

// effectiveTags is the set of tags a resource must carry: the provider's
// default tags plus the declared ones.
func effectiveTags(ctx context.Context, declared []string) []string {
	return normalizeTags(append(append([]string{}, providerDefaultTags(ctx)...), declared...))
}

// tagsDiffer compares two tag sets regardless of order and spelling.
func tagsDiffer(a, b []string) bool {
	a, b = normalizeTags(a), normalizeTags(b)
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

// reconcileTags attaches the desired tags that are missing on the owner and
// detaches the tags the provider applied before that are no longer desired.
// Tags attached outside Pulumi are left untouched. It returns the applied set.
func reconcileTags(ctx context.Context, c *coolify.Client, owner coolify.Owner, desired, previousApplied []string) ([]string, error) {
	desired = normalizeTags(desired)
	present, err := c.ListTags(ctx, owner)
	if err != nil {
		return nil, err
	}
	byName := map[string]coolify.Tag{}
	for _, tag := range present {
		byName[normalizeTag(tag.Name)] = tag
	}
	var missing []string
	for _, name := range desired {
		if _, ok := byName[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		if _, err := c.AttachTags(ctx, owner, missing); err != nil {
			return nil, err
		}
	}
	wanted := map[string]struct{}{}
	for _, name := range desired {
		wanted[name] = struct{}{}
	}
	for _, name := range normalizeTags(previousApplied) {
		if _, ok := wanted[name]; ok {
			continue
		}
		if tag, ok := byName[name]; ok {
			if err := c.DetachTag(ctx, owner, tag.UUID); err != nil && !coolify.IsNotFound(err) {
				return nil, err
			}
		}
	}
	return desired, nil
}

// readTags derives the declared tags that are still attached and the applied
// tags that are still attached, so tags detached in Coolify are re-attached
// on the next update.
func readTags(ctx context.Context, c *coolify.Client, owner coolify.Owner, declared, previousApplied []string) (tags, applied []string, err error) {
	present, err := c.ListTags(ctx, owner)
	if err != nil {
		return nil, nil, err
	}
	attached := map[string]struct{}{}
	for _, tag := range present {
		attached[normalizeTag(tag.Name)] = struct{}{}
	}
	keep := func(names []string) []string {
		out := []string{}
		for _, name := range normalizeTags(names) {
			if _, ok := attached[name]; ok {
				out = append(out, name)
			}
		}
		return out
	}
	tags = keep(declared)
	if len(declared) == 0 {
		tags = nil
	}
	return tags, keep(previousApplied), nil
}
