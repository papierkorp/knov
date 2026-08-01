// Package dashboard handles dashboard operations
package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"

	"knov/internal/configStorage"
	"knov/internal/keylock"
	"knov/internal/logging"
	"knov/internal/utils"
)

// dashboardLocks guards Mutate's read-modify-write span per dashboard id - see internal/keylock.
var dashboardLocks = keylock.New()

// ErrNotFound is returned by Get (and so by Mutate) when id has no dashboard - callers use
// errors.Is to tell a missing dashboard apart from other failures (e.g. bad form input).
var ErrNotFound = errors.New("dashboard not found")

// Layout represents dashboard layout types
type Layout string

const (
	OneColumn    Layout = "oneColumn"
	TwoColumns   Layout = "twoColumns"
	ThreeColumns Layout = "threeColumns"
	FourColumns  Layout = "fourColumns"
	Custom       Layout = "custom"
)

// Dashboard represents a dashboard structure
type Dashboard struct {
	Name    string   `json:"name"`
	ID      string   `json:"id"`
	Layout  Layout   `json:"layout"`
	Widgets []Widget `json:"widgets"`
}

// GetAll returns all dashboards
func GetAll() ([]Dashboard, error) {
	var dashboards []Dashboard

	// Get all dashboards from global storage
	globalKeys, err := configStorage.List("dashboard/")
	if err != nil {
		return nil, err
	}

	for _, key := range globalKeys {
		data, err := configStorage.Get(key)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "failed to get dashboard %s: %v", key, err)
			continue
		}

		var dashboard Dashboard
		if err := json.Unmarshal(data, &dashboard); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to unmarshal dashboard %s: %v", key, err)
			continue
		}

		dashboards = append(dashboards, dashboard)
	}

	logging.LogDebug(logging.KeyApp, "retrieved %d dashboards", len(dashboards))
	return dashboards, nil
}

// Get returns a specific dashboard
func Get(id string) (*Dashboard, error) {
	key := fmt.Sprintf("dashboard/%s", id)
	data, err := configStorage.Get(key)

	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}

	var dashboard Dashboard
	if err := json.Unmarshal(data, &dashboard); err != nil {
		return nil, err
	}

	logging.LogDebug(logging.KeyApp, "retrieved dashboard: %s", id)
	return &dashboard, nil
}

// Create creates a new dashboard
func Create(dashboard *Dashboard) error {
	dashboard.ID = utils.CleanseID(dashboard.Name)

	// Check if dashboard already exists
	existing, _ := Get(dashboard.ID)
	if existing != nil {
		return fmt.Errorf("dashboard with id '%s' already exists", dashboard.ID)
	}

	// Validate layout
	if !isValidLayout(dashboard.Layout) {
		return fmt.Errorf("invalid layout: %s", dashboard.Layout)
	}

	if dashboard.Widgets == nil {
		dashboard.Widgets = []Widget{}
	}

	// Auto-generate widget IDs
	for i := range dashboard.Widgets {
		if dashboard.Widgets[i].ID == "" {
			dashboard.Widgets[i].ID = fmt.Sprintf("widget-%d", i)
		}
	}

	data, err := json.Marshal(dashboard)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("dashboard/%s", dashboard.ID)
	if err := configStorage.Set(key, data); err != nil {
		return err
	}

	logging.LogDebug(logging.KeyApp, "created dashboard: %s", dashboard.ID)
	return nil
}

// Mutate loads id's dashboard under its own lock, lets fn modify it in place, and saves the
// result before releasing the lock - so a rename racing a widget-layout edit on the same
// dashboard can't read stale data and revert each other (same race MetaDataMutate closes for
// file metadata, see internal/files/metadata.go).
func Mutate(id string, fn func(d *Dashboard) error) (*Dashboard, error) {
	unlock := dashboardLocks.Lock(id)
	defer unlock()

	dash, err := Get(id)
	if err != nil {
		return nil, err
	}
	if err := fn(dash); err != nil {
		return nil, err
	}
	if err := Update(dash); err != nil {
		return nil, err
	}
	return dash, nil
}

// Update updates an existing dashboard, replacing it wholesale. Only safe for callers that own
// the entire record (e.g. test setup) - a caller that read the dashboard first and wants to
// change part of it must use Mutate instead, or a concurrent edit can read stale data and get
// silently reverted.
func Update(dashboard *Dashboard) error {
	// Validate layout
	if !isValidLayout(dashboard.Layout) {
		return fmt.Errorf("invalid layout: %s", dashboard.Layout)
	}

	data, err := json.Marshal(dashboard)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("dashboard/%s", dashboard.ID)
	if err := configStorage.Set(key, data); err != nil {
		return err
	}

	logging.LogDebug(logging.KeyApp, "updated dashboard: %s", dashboard.ID)
	return nil
}

// isValidLayout checks if the layout is one of the allowed enum values
func isValidLayout(layout Layout) bool {
	switch layout {
	case OneColumn, TwoColumns, ThreeColumns, FourColumns, Custom:
		return true
	default:
		return false
	}
}

// Delete removes a dashboard
func Delete(id string) error {
	existing, _ := Get(id)
	if existing == nil {
		return fmt.Errorf("dashboard with id '%s' not found", id)
	}

	key := fmt.Sprintf("dashboard/%s", id)
	if err := configStorage.Delete(key); err != nil {
		return err
	}

	logging.LogDebug(logging.KeyApp, "deleted dashboard: %s", id)
	return nil
}
