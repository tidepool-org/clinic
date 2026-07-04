package postgres

// BatchResults is the surface of sqlc-generated :batchexec results.
type BatchResults interface {
	Exec(func(int, error))
	Close() error
}

// ExecBatch drains a batch and returns the first error.
func ExecBatch(results BatchResults) error {
	var first error
	results.Exec(func(_ int, err error) {
		if err != nil && first == nil {
			first = err
		}
	})
	if err := results.Close(); err != nil && first == nil {
		first = err
	}
	return first
}
