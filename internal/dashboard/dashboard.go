// Package dashboard handles dashboard operations
package dashboard

import (
	"encoding/json"
	"fmt"

	"knov/internal/logging"
	"knov/internal/storage"
	"knov/internal/utils"
)

// Layout represents dashboard layout types
type Layout string

const (
	OneColumn    Layout = "oneColumn"
	TwoColumns   Layout = "twoColumns"
	ThreeColumns Layout = "threeColumns"
	FourColumns  Layout = "fourColumns"
)

// WidgetPosition represents widget position on dashboard
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Widget represents a dashboard widget
type Widget struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Position WidgetPosition `json:"position"`
	Config   interface{}    `json:"config"`
}

// Dashboard represents a dashboard structure
type Dashboard struct {
	Name    string   `json:"name"`
	ID      string   `json:"id"`
	Layout  Layout   `json:"layout"`
	Widgets []Widget `json:"widgets"`
	Global  bool     `json:"global"`
}

// GetAll returns all dashboards for user or global
func GetAll(userID string) ([]Dashboard, error) {
	var dashboards []Dashboard

	// Get global dashboards
	globalKeys, err := storage.GetStorage().List("dashboard/")
	if err != nil {
		return nil, err
	}

	for _, key := range globalKeys {
		data, err := storage.GetStorage().Get(key)
		if err != nil {
			logging.LogWarning("failed to get dashboard %s: %v", key, err)
			continue
		}

		var dashboard Dashboard
		if err := json.Unmarshal(data, &dashboard); err != nil {
			logging.LogWarning("failed to unmarshal dashboard %s: %v", key, err)
			continue
		}

		dashboards = append(dashboards, dashboard)
	}

	// Get user dashboards if not global
	userPrefix := fmt.Sprintf("user/%s/dashboard/", userID)
	userKeys, err := storage.GetStorage().List(userPrefix)
	if err == nil {
		for _, key := range userKeys {
			data, err := storage.GetStorage().Get(key)
			if err != nil {
				logging.LogWarning("failed to get user dashboard %s: %v", key, err)
				continue
			}

			var dashboard Dashboard
			if err := json.Unmarshal(data, &dashboard); err != nil {
				logging.LogWarning("failed to unmarshal user dashboard %s: %v", key, err)
				continue
			}

			dashboards = append(dashboards, dashboard)
		}
	}

	logging.LogDebug("retrieved %d dashboards", len(dashboards))
	return dashboards, nil
}

// Get returns a specific dashboard
func Get(id string, userID string) (*Dashboard, error) {
	// Try global first
	key := fmt.Sprintf("dashboard/%s", id)
	data, err := storage.GetStorage().Get(key)

	// If not found globally, try user-specific
	if data == nil && err == nil {
		key = fmt.Sprintf("user/%s/dashboard/%s", userID, id)
		data, err = storage.GetStorage().Get(key)
	}

	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, fmt.Errorf("dashboard with id '%s' not found", id)
	}

	var dashboard Dashboard
	if err := json.Unmarshal(data, &dashboard); err != nil {
		return nil, err
	}

	logging.LogDebug("retrieved dashboard: %s", id)
	return &dashboard, nil
}

// Create creates a new dashboard
func Create(dashboard *Dashboard, userID string) error {
	dashboard.ID = utils.CleanseID(dashboard.Name)

	// Check if dashboard already exists
	existing, _ := Get(dashboard.ID, userID)
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

	data, err := json.Marshal(dashboard)
	if err != nil {
		return err
	}

	var key string
	if dashboard.Global {
		key = fmt.Sprintf("dashboard/%s", dashboard.ID)
	} else {
		key = fmt.Sprintf("user/%s/dashboard/%s", userID, dashboard.ID)
	}

	if err := storage.GetStorage().Set(key, data); err != nil {
		return err
	}

	logging.LogDebug("created dashboard: %s", dashboard.ID)
	return nil
}

// Update updates an existing dashboard
func Update(dashboard *Dashboard, userID string) error {
	// Validate layout
	if !isValidLayout(dashboard.Layout) {
		return fmt.Errorf("invalid layout: %s", dashboard.Layout)
	}

	data, err := json.Marshal(dashboard)
	if err != nil {
		return err
	}

	var key string
	if dashboard.Global {
		key = fmt.Sprintf("dashboard/%s", dashboard.ID)
	} else {
		key = fmt.Sprintf("user/%s/dashboard/%s", userID, dashboard.ID)
	}

	if err := storage.GetStorage().Set(key, data); err != nil {
		return err
	}

	logging.LogDebug("updated dashboard: %s", dashboard.ID)
	return nil
}

// isValidLayout checks if the layout is one of the allowed enum values
func isValidLayout(layout Layout) bool {
	switch layout {
	case OneColumn, TwoColumns, ThreeColumns, FourColumns:
		return true
	default:
		return false
	}
}

// Delete removes a dashboard
func Delete(id string, userID string) error {
	// Check if dashboard exists first
	existing, _ := Get(id, userID)
	if existing == nil {
		return fmt.Errorf("dashboard with id '%s' not found", id)
	}

	var key string
	if existing.Global {
		key = fmt.Sprintf("dashboard/%s", id)
	} else {
		key = fmt.Sprintf("user/%s/dashboard/%s", userID, id)
	}

	if err := storage.GetStorage().Delete(key); err != nil {
		return err
	}

	logging.LogDebug("deleted dashboard: %s", id)
	return nil
}
