// Package git - Git operations for file management
package git

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// HandleAPIGetRecentlyChanged returns recently changed files
func HandleAPIGetRecentlyChanged(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 10 // default
	if countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil {
			count = c
		}
	}

	files, err := GetRecentlyChangedFiles(count)
	if err != nil {
		http.Error(w, "failed to get recent files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}
