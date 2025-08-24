// Package server ..
package server

import (
	"encoding/json"
	"net/http"

	"knov/internal/configmanager"
)

// @Summary Get current configuration
// @Tags config
// @Router /api/config/getConfig [get]
func handleAPIGetConfig(w http.ResponseWriter, r *http.Request) {
	config := configmanager.GetConfig()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// @Summary Set configuration
// @Tags config
// @Router /api/config/setConfig [post]
func handleAPISetConfig(w http.ResponseWriter, r *http.Request) {
	var config configmanager.ConfigManager

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	configmanager.SetConfig(config)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
