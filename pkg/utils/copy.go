package utils

func CopyIntP(i *int) *int {
	if i == nil {
		return nil
	}
	c := *i
	return &c
}

func CopyFloat64P(f *float64) *float64 {
	if f == nil {
		return nil
	}
	c := *f
	return &c
}

func CopyBoolP(b *bool) *bool {
	if b == nil {
		return nil
	}
	c := *b
	return &c
}

func CopyAnyP(a *any) *any {
	if a == nil {
		return nil
	}
	c := *a
	return &c
}

func CopyStringArray(slice []string) []string {
	if slice == nil {
		return nil
	}
	dst := make([]string, len(slice))
	copy(dst, slice)
	return dst
}

func CopyAnyArray(slice []any) []any {
	if slice == nil {
		return nil
	}
	dst := make([]any, len(slice))
	copy(dst, slice)
	return dst
}

func CopyMapAny(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	dst := make(map[string]any, len(m))
	for k, v := range m {
		dst[k] = v
	}
	return dst
}

func CopyMapString(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	dst := make(map[string]string, len(m))
	for k, v := range m {
		dst[k] = v
	}
	return dst
}

func CopyMapStringArray(m map[string][]string) map[string][]string {
	if m == nil {
		return nil
	}
	dst := make(map[string][]string, len(m))
	for k, v := range m {
		dst[k] = CopyStringArray(v)
	}
	return dst
}
