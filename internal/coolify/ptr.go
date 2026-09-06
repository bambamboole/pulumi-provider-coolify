package coolify

// Ptr returns a pointer to v. Handy for the pointer-typed fields of generated
// request bodies.
func Ptr[T any](v T) *T { return &v }

// Deref returns the value p points to, or the zero value when p is nil.
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// PtrIfNonZero returns a pointer to v, or nil when v is the zero value. Use it
// to omit unset optional inputs from request bodies.
func PtrIfNonZero[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}
