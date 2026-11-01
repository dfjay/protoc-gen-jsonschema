package utils

import "slices"

// ClonePtr returns a pointer to a copy of *p, or nil when p is nil.
func ClonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	return new(*p)
}

// CloneMapSlices copies m and each slice in it.
func CloneMapSlices[K comparable, V any](m map[K][]V) map[K][]V {
	if m == nil {
		return nil
	}
	dst := make(map[K][]V, len(m))
	for k, v := range m {
		dst[k] = slices.Clone(v)
	}
	return dst
}
