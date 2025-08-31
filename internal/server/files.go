package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"knov/internal/logging"
)

// FileInfo ..
type FileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url"`
}

func handleAPIGetFiles(w http.ResponseWriter, r *http.Request) {
	var files []FileInfo

	err := filepath.Walk("data", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".md") {
			files = append(files, FileInfo{
				Name: info.Name(),
				Path: path,
				URL:  "/markdown/" + strings.TrimPrefix(path, "data/"),
			})
		}
		return nil
	})

	logging.LogDebug("handleAPIGetFiles: %v", files)

	if err != nil {
		logging.LogError("failed to read data directory: %v", err)
		http.Error(w, "failed to read files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	if len(files) == 0 {
		w.Write([]byte("no markdown files found."))
		return
	}

	w.Write([]byte("<ul>"))
	for _, file := range files {
		w.Write([]byte("<li><a href=\"#\" hx-get=\"" + file.URL + "\" hx-target=\"#content-area\">" + file.Name + "</a></li>"))
	}
	w.Write([]byte("</ul>"))
}

func handleAPIMarkdown(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/markdown/")
	fullPath := filepath.Join("data", filePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, "file not found", http.StatusInternalServerError)
		return
	}

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	parser := parser.NewWithExtensions(extensions)
	html := markdown.ToHTML(content, parser, nil)

	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
}
