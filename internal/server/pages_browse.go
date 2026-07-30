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
	"knov/internal/thememanager"

	"github.com/go-chi/chi/v5"
)

func handleSearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	tm := thememanager.GetThemeManager()
	data := thememanager.NewSearchPageData(query)

	err := tm.Render(w, "search", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleBrowseFiles(w http.ResponseWriter, r *http.Request) {
	metadataType := chi.URLParam(r, "metadata")
	value := chi.URLParam(r, "value")

	if metadataType == "" || value == "" {
		http.Error(w, "missing metadata type or value", http.StatusBadRequest)
		return
	}

	tm := thememanager.GetThemeManager()
	title := fmt.Sprintf("Browse: %s", value)
	data := thememanager.NewBrowseFilesTemplateData(metadataType, value)
	data.Title = title

	err := tm.Render(w, "browsefiles", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleBrowse(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewBaseTemplateData("Browse")

	err := tm.Render(w, "browse", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleBrowseMedia(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := thememanager.NewMediaOverviewTemplateData()

	err := tm.Render(w, "mediaoverview", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleRedirectToBrowseMedia(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/browse/media", http.StatusPermanentRedirect)
}

func handleRedirectToBrowseFiles(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/browse/files", http.StatusPermanentRedirect)
}

func handleMedia(w http.ResponseWriter, r *http.Request) {
	mediaPath := chi.URLParam(r, "*")
	if mediaPath == "" {
		http.NotFound(w, r)
		return
	}

	if strings.HasPrefix(mediaPath, "http://") || strings.HasPrefix(mediaPath, "https://") {
		http.Redirect(w, r, mediaPath, http.StatusPermanentRedirect)
		return
	}

	fullPath := pathutils.ToMediaPath(mediaPath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		logging.LogWarning(logging.KeyApp, "media file not found: %s", fullPath)
		http.NotFound(w, r)
		return
	}

	if r.URL.Query().Get("mode") == "detail" {
		tm := thememanager.GetThemeManager()
		data := thememanager.NewMediaViewTemplateData(mediaPath)

		err := tm.Render(w, "mediaview", data)
		if err != nil {
			http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
			return
		}
		return
	}

	ext := strings.ToLower(filepath.Ext(mediaPath))
	if ct := configmanager.MimeTypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000")

	logging.LogDebug(logging.KeyApp, "serving media file: %s", fullPath)
	http.ServeFile(w, r, fullPath)
}

func handleBrowseMetadata(w http.ResponseWriter, r *http.Request) {
	metadataType := chi.URLParam(r, "metadata")

	if metadataType == "" {
		http.Error(w, "missing metadata type", http.StatusBadRequest)
		return
	}

	tm := thememanager.GetThemeManager()
	data := thememanager.NewBrowseMetadataTemplateData(metadataType)

	err := tm.Render(w, "browsemetadata", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}
