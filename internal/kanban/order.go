package kanban

import (
	"encoding/json"
	"fmt"

	"knov/internal/configStorage"
	"knov/internal/logging"
)

// Order maps status → ordered list of file paths for one board folder.
type Order map[string][]string

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
		logging.LogWarning("kanban: corrupt order for folder %s, resetting: %v", folderPath, err)
		return Order{}, nil
	}
	return o, nil
}

// SaveOrder persists the card order for a board folder.
func SaveOrder(folderPath string, o Order) error {
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Errorf("kanban: marshal order failed: %w", err)
	}
	return configStorage.Set(orderKey(folderPath), data)
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
