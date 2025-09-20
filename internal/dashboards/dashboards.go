// Package dashboards handles dashboard operations
package dashboards

import (
	"encoding/json"
	"fmt"

	"knov/internal/logging"
	"knov/internal/storage"
)

type Dashboard struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Layout  string            `json:"layout"`
	Widgets []DashboardWidget `json:"widgets"`
}

type DashboardWidget struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Position map[string]interface{} `json:"position"`
	Filter   []interface{}          `json:"filter"`
}

// GetAll returns all dashboards
func GetAll() ([]Dashboard, error) {
	keys, err := storage.GetStorage().List("dashboard/")
	if err != nil {
		logging.LogError("failed to list dashboards: %v", err)
		return nil, err
	}

	var dashboards []Dashboard
	for _, key := range keys {
		dashboard, err := GetByKey(key)
		if err != nil {
			logging.LogWarning("failed to load dashboard %s: %v", key, err)
			continue
		}
		dashboards = append(dashboards, *dashboard)
	}

	return dashboards, nil
}

// GetByID returns dashboard by ID
func GetByID(id string) (*Dashboard, error) {
	key := "dashboard/" + id
	return GetByKey(key)
}

// GetByKey returns dashboard by storage key
func GetByKey(key string) (*Dashboard, error) {
	data, err := storage.GetStorage().Get(key)
	if err != nil || data == nil {
		return nil, fmt.Errorf("dashboard not found")
	}

	var dashboard Dashboard
	if err := json.Unmarshal(data, &dashboard); err != nil {
		return nil, fmt.Errorf("failed to parse dashboard: %v", err)
	}

	return &dashboard, nil
}

// Save saves a dashboard
func Save(dashboard *Dashboard) error {
	if dashboard.ID == "" {
		return fmt.Errorf("dashboard id is required")
	}

	if dashboard.Layout == "" {
		dashboard.Layout = "single-column"
	}

	if dashboard.Widgets == nil {
		dashboard.Widgets = []DashboardWidget{}
	}

	data, err := json.Marshal(dashboard)
	if err != nil {
		return fmt.Errorf("failed to marshal dashboard: %v", err)
	}

	key := "dashboard/" + dashboard.ID
	if err := storage.GetStorage().Set(key, data); err != nil {
		return fmt.Errorf("failed to save dashboard: %v", err)
	}

	logging.LogDebug("dashboard saved: %s", dashboard.ID)
	return nil
}

// Delete deletes a dashboard
func Delete(id string) error {
	key := "dashboard/" + id
	if err := storage.GetStorage().Delete(key); err != nil {
		return fmt.Errorf("failed to delete dashboard: %v", err)
	}

	logging.LogDebug("dashboard deleted: %s", id)
	return nil
}

// Exists checks if dashboard exists
func Exists(id string) bool {
	key := "dashboard/" + id
	return storage.GetStorage().Exists(key)
}
