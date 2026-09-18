package trackerStorage

type noopStorage struct{}

func (n *noopStorage) GetDays(_, _ string) (map[string]int, error) { return map[string]int{}, nil }

func (n *noopStorage) AddDelta(_, _, _ string, _ int) error { return nil }

func (n *noopStorage) ResetCounter(_, _ string) error { return nil }

func (n *noopStorage) DeleteTracker(_ string) error { return nil }

func (n *noopStorage) GetBackendType() string { return "noop" }

func (n *noopStorage) Backup(_ string) error { return nil }

func (n *noopStorage) Restore(_ string) error { return nil }
