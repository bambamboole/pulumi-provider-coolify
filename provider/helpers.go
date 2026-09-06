package provider

import (
	"context"
	"reflect"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// client returns the Coolify API client configured by the provider.
func client(ctx context.Context) *coolify.Client {
	return infer.GetConfig[Config](ctx).client
}

// diffArgs compares the pulumi-tagged fields of old and new argument structs
// and returns a property diff per changed field. Fields listed in replaces
// trigger a replacement instead of an update. Nil and empty slices or maps are
// considered equal, so an omitted list never diffs against an empty one.
func diffArgs[A any](old, new A, replaces ...string) map[string]p.PropertyDiff {
	diff := map[string]p.PropertyDiff{}
	oldValue, newValue := reflect.ValueOf(old), reflect.ValueOf(new)
	typ := oldValue.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("pulumi")
		if tag == "" || !field.IsExported() {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if equalValues(oldValue.Field(i), newValue.Field(i)) {
			continue
		}
		kind := p.Update
		for _, replace := range replaces {
			if replace == name {
				kind = p.UpdateReplace
			}
		}
		diff[name] = p.PropertyDiff{Kind: kind}
	}
	return diff
}

func equalValues(a, b reflect.Value) bool {
	switch a.Kind() {
	case reflect.Slice, reflect.Map:
		if a.Len() == 0 && b.Len() == 0 {
			return true
		}
	}
	return reflect.DeepEqual(a.Interface(), b.Interface())
}

// diffResponse builds the infer response for a detailed diff. Replacements are
// performed delete-before-create when sameIdentity is true, because Create
// adopts existing Coolify resources by name and would otherwise adopt the
// resource that is about to be deleted.
func diffResponse(diff map[string]p.PropertyDiff, sameIdentity bool) infer.DiffResponse {
	replaces := false
	for _, d := range diff {
		if d.Kind == p.UpdateReplace {
			replaces = true
		}
	}
	return infer.DiffResponse{
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
		DeleteBeforeReplace: replaces && sameIdentity,
	}
}

// uniquePreservingOrder returns the deduplicated values in first-occurrence order.
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

// ifSet returns value when the previous input was set (managed) and keeps the
// previous input otherwise. Read uses it so optional inputs the program leaves
// unset are not overwritten with the values Coolify defaulted them to.
func ifSet[T comparable](previous, value T) T {
	var zero T
	if previous == zero {
		return zero
	}
	return value
}

// patch tracks whether any field of a PATCH body was set. Optional string
// inputs left empty are treated as unmanaged and never sent, so Coolify keeps
// its own value; booleans and required strings are always reconciled.
type patch struct{ changed bool }

// str sets dst when desired is set and differs from current.
func (p *patch) str(dst **string, desired, current string) {
	if desired != "" && desired != current {
		*dst = &desired
		p.changed = true
	}
}

// text sets dst whenever desired differs from current, including clearing it.
func (p *patch) text(dst **string, desired, current string) {
	if desired != current {
		*dst = &desired
		p.changed = true
	}
}

// boolean sets dst when desired differs from current.
func (p *patch) boolean(dst **bool, desired, current bool) {
	if desired != current {
		*dst = &desired
		p.changed = true
	}
}

// integer sets dst when desired is set and differs from current.
func (p *patch) integer(dst **int, desired int, current *int) {
	if desired != 0 && (current == nil || *current != desired) {
		*dst = &desired
		p.changed = true
	}
}

// optionalInt sets dst when desired is set and differs from current.
func (p *patch) optionalInt(dst **int, desired, current *int) {
	if desired != nil && (current == nil || *current != *desired) {
		*dst = desired
		p.changed = true
	}
}
