// Package server ..
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/swaggo/http-swagger/v2"
	_ "knov/docs" // swaggo api docs
)

func apiRoutes(r chi.Router) {
	r.Get("/swagger/*", httpSwagger.Handler())
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handleAPIHealth)
		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- THEMES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/themes", func(r chi.Router) {
			r.Get("/getAllThemes", handleAPIGetThemes)
			r.Post("/setTheme", handleAPISetTheme)

		})
		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- THEMES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/config", func(r chi.Router) {
			r.Get("/getConfig", handleAPIGetConfig)
			r.Post("/setConfig", handleAPISetConfig)
		})

	})
}

// @Summary Health check
// @Tags health
// @Router /api/health [get]
func handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
