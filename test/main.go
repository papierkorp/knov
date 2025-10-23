package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	InitThemeManager()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", handleBase)
	r.Post("/", handleThemeChange)

	http.ListenAndServe(":1325", r)
}

func handleBase(w http.ResponseWriter, r *http.Request) {
}

func handleThemeChange(w http.ResponseWriter, r *http.Request) {
}
