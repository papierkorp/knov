// Package server - API utility functions
package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

func writeResponse(w http.ResponseWriter, r *http.Request, jsonData any, htmlData string) {
	acceptHeader := r.Header.Get("Accept")

	if strings.Contains(acceptHeader, "text/html") || strings.Contains(acceptHeader, "*/*") {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlData))
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonData)
	}
}
