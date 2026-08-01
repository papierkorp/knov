package kanban

import (
	"encoding/json"
	"fmt"

	"knov/internal/configStorage"
	"knov/internal/keylock"
	"knov/internal/logging"
)

// Order maps status → ordered list of file paths for one board folder.
type Order map[string][]string

// orderLocks guards MutateOrder's read-modify-write span per board folder - see internal/keylock.
var orderLocks = keylock.New()

func orderKey(folderPath string) string {
	return fmt.Sprintf("kanban-order/%s", folderPath)
}

// GetOrder loads the stored card order for a board folder.
func GetOrder(folderPath string) (Order, error) {
	data, err := configStorage.Get(orderKey(folderPath))
	if err != nil {
		return Order{}, err
	}
	if data == nil {
		return Order{}, nil
	}
	var o Order
	if err := json.Unmarshal(data, &o); err != nil {
		logging.LogWarning(logging.KeyApp, "kanban: corrupt order for folder %s, resetting: %v", folderPath, err)
		return Order{}, nil
	}
	return o, nil
}

// SaveOrder persists the card order for a board folder, replacing it wholesale. Only safe for
// callers that own the entire record (e.g. test setup) - a caller that read the order first and
// wants to change part of it must use MutateOrder instead, or a concurrent reorder of another
// column on the same board can read stale data and get silently reverted.
func SaveOrder(folderPath string, o Order) error {
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Errorf("kanban: marshal order failed: %w", err)
	}
	return configStorage.Set(orderKey(folderPath), data)
}

// MutateOrder loads folderPath's order under its own lock, lets fn modify it in place, and
// saves the result before releasing the lock - so a concurrent reorder of a different column on
// the same board can't read stale data and revert this one (same race MetaDataMutate closes for
// file metadata, see internal/files/metadata.go).
func MutateOrder(folderPath string, fn func(o Order)) error {
	unlock := orderLocks.Lock(folderPath)
	defer unlock()

	o, err := GetOrder(folderPath)
	if err != nil {
		o = Order{}
	}
	fn(o)
	return SaveOrder(folderPath, o)
}

// ApplyOrder reorders cards according to stored order.
// Cards not present in stored are appended at the end in their original sequence.
func ApplyOrder(stored []string, cards []string) []string {
	if len(stored) == 0 {
		return cards
	}

	pos := make(map[string]int, len(stored))
	for i, fp := range stored {
		pos[fp] = i
	}

	known := make([]string, 0, len(cards))
	unknown := make([]string, 0)
	for _, c := range cards {
		if _, ok := pos[c]; ok {
			known = append(known, c)
		} else {
			unknown = append(unknown, c)
		}
	}

	// insertion sort by stored position
	for i := 1; i < len(known); i++ {
		key := known[i]
		j := i - 1
		for j >= 0 && pos[known[j]] > pos[key] {
			known[j+1] = known[j]
			j--
		}
		known[j+1] = key
	}

	return append(known, unknown...)
}
