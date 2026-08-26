package kanbanStorage

import "time"

type noopStorage struct{}

func (n *noopStorage) LogEvent(_, _, _, _ string) error {
	return nil
}

func (n *noopStorage) GetEvents(_, _ string, _, _ *time.Time, _ int) ([]Event, error) {
	return []Event{}, nil
}

func (n *noopStorage) insertEvents(_ []Event) error { return nil }

func (n *noopStorage) GetBackendType() string { return "noop" }

func (n *noopStorage) Cleanup() error { return nil }

func (n *noopStorage) Backup(_ string) error { return nil }

func (n *noopStorage) Restore(_ string) error { return nil }
