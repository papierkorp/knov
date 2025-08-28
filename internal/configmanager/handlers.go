// Package configmanager - General configuration handlers
package configmanager

import (
	"encoding/json"
	"net/http"
)

// HandleGetConfig returns current configuration
func HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	config := GetConfig()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// HandleSetConfig updates configuration
func HandleSetConfig(w http.ResponseWriter, r *http.Request) {
	var config ConfigManager

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	SetConfig(config)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleSetLanguage sets the language from form data
func HandleSetLanguage(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	lang := r.FormValue("language")

	if lang != "" {
		SetLanguage(lang)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
