package pointer

func FromAny[T any](v T) *T {
	return &v
}

func ToString(p *string) string {
	if p == nil {
		return ""
	}

	return *p
}
