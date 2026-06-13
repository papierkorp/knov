package kanbanStorage

import "time"

type noopStorage struct{}

func (n *noopStorage) LogEvent(_, _, _, _ string) error {
	return nil
}

func (n *noopStorage) GetEvents(_, _ string, _, _ *time.Time, _ int) ([]Event, error) {
	return []Event{}, nil
}
