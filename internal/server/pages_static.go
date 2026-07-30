package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/pathutils"
)

// handleFavicon serves the custom favicon if one has been uploaded,
// otherwise falls back to the embedded static/favicon.ico.
func handleFavicon(w http.ResponseWriter, r *http.Request) {
	customPath := configmanager.GetCustomFaviconPath()
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			ext := strings.ToLower(filepath.Ext(customPath))
			switch ext {
			case ".svg":
				w.Header().Set("Content-Type", "image/svg+xml")
			case ".png":
				w.Header().Set("Content-Type", "image/png")
			default:
				w.Header().Set("Content-Type", "image/x-icon")
			}
			http.ServeFile(w, r, customPath)
			return
		}
	}
	// fall back to embedded default favicon
	r.URL.Path = "/static/favicon.ico"
	handleStatic(w, r)
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	var basePath, filePath, fullPath string

	if strings.HasPrefix(r.URL.Path, "/static/") {
		basePath = "static"
		filePath = strings.TrimPrefix(r.URL.Path, "/static/")
		fullPath = pathutils.ToSlash(filepath.Join(basePath, filePath))
	} else if strings.HasPrefix(r.URL.Path, "/themes/") {
		basePath = "themes"
		filePath = strings.TrimPrefix(r.URL.Path, "/themes/")
		fullPath = filepath.Join(basePath, filePath)
	} else {
		http.NotFound(w, r)
		return
	}

	if basePath == "static" && strings.HasPrefix(filePath, "css/") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		cssFile := strings.TrimPrefix(filePath, "css/")

		if cssFile == "custom.css" {
			w.Write([]byte(configmanager.GetCustomCSS()))
			return
		}
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	// set content type headers before serving files
	if ct := configmanager.MimeTypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	if basePath == "themes" {
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			logging.LogDebug(logging.KeyApp, "theme file not found: %s", fullPath)
			http.NotFound(w, r)
			return
		}

		// for CSS files, read and serve manually to ensure correct MIME type
		if ext == ".css" {
			cssData, err := os.ReadFile(fullPath)
			if err != nil {
				logging.LogError(logging.KeyApp, "failed to read theme CSS file %s: %v", fullPath, err)
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Write(cssData)
			logging.LogDebug(logging.KeyApp, "serving theme CSS file: %s", fullPath)
			return
		}

		logging.LogDebug(logging.KeyApp, "serving theme file: %s", fullPath)
		http.ServeFile(w, r, fullPath)
	} else {
		data, err := staticFiles.ReadFile(fullPath)
		if err != nil {
			fmt.Printf("failed to read embedded file %s: %v\n", fullPath, err)
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}
}

// handleWebfontsRedirect redirects /webfonts/* requests to /static/font-awesome/webfonts/7-3-1/*
func handleWebfontsRedirect(w http.ResponseWriter, r *http.Request) {
	fontPath := strings.TrimPrefix(r.URL.Path, "/webfonts/")
	newPath := "/static/font-awesome/webfonts/7-3-1/" + fontPath

	newURL := *r.URL
	newURL.Path = newPath

	newReq := r.Clone(r.Context())
	newReq.URL = &newURL

	handleStatic(w, newReq)
}
