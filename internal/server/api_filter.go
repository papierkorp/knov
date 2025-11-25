// Package server - Filter API handlers
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Filter files by metadata
// @Description Filter files based on metadata criteria with configurable logic and display
// @Tags filter
// @Accept application/x-www-form-urlencoded
// @Param metadata[] formData array false "Metadata field names"
// @Param operator[] formData array false "Filter operators (equals, contains, greater, less, in)"
// @Param value[] formData array false "Filter values"
// @Param action[] formData array false "Filter actions (include, exclude)"
// @Param logic formData string false "Logic operator (and/or)" default(and)
// @Param display formData string false "Display type (list, cards, dropdown, table)" default(list)
// @Param limit formData int false "Maximum number of results" default(50)
// @Produce json,html
// @Success 200 {object} filter.Result
// @Router /api/filter [post]
func handleAPIFilterFiles(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug("filter request received")

	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	config := filter.ParseFilterConfigFromForm(r)
	if err := filter.ValidateConfig(config); err != nil {
		logging.LogError("invalid filter config: %v", err)
		http.Error(w, fmt.Sprintf(translation.SprintfForRequest(configmanager.GetLanguage(), "invalid filter config: %v"), err), http.StatusBadRequest)
		return
	}

	logging.LogDebug("built filter config: %+v", config)

	result, err := filter.FilterFilesWithConfig(config)
	if err != nil {
		logging.LogError("failed to filter files: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to filter files"), http.StatusInternalServerError)
		return
	}

	logging.LogDebug("filtered %d files from %d total", len(result.Files), result.Total)

	html := render.RenderFilterResult(result, config.Display)
	writeResponse(w, r, result, html)
}

// @Summary Get filter criteria row
// @Description Get HTML for a new filter criteria row
// @Tags filter
// @Accept application/x-www-form-urlencoded
// @Param row_index formData int false "Row index"
// @Produce text/html
// @Success 200 {string} string "filter criteria row html"
// @Router /api/filter/criteria-row [get]
func handleAPIGetFilterCriteriaRow(w http.ResponseWriter, r *http.Request) {
	indexStr := r.URL.Query().Get("row_index")
	if indexStr == "" {
		indexStr = r.FormValue("row_index")
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		index = 0
	}

	html := render.RenderFilterCriteriaRow(index, nil)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Save filter configuration
// @Description Save filter configuration as JSON file with .filter extension
// @Tags filter
// @Accept application/x-www-form-urlencoded
// @Param filepath formData string false "Filter file path (without extension, optional for new files)"
// @Param metadata[] formData array false "Metadata field names"
// @Param operator[] formData array false "Filter operators (equals, contains, greater, less, in)"
// @Param value[] formData array false "Filter values"
// @Param action[] formData array false "Filter actions (include, exclude)"
// @Param logic formData string false "Logic operator (and/or)" default(and)
// @Produce html
// @Success 200 {string} string "success message"
// @Router /api/filter/save [post]
func handleAPIFilterSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	// get file path
	filePath := r.FormValue("filepath")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	// parse filter config from form using existing function
	config := filter.ParseFilterConfigFromForm(r)

	// save using the new filter package function
	if err := filter.SaveFilterConfig(config, filePath); err != nil {
		logging.LogError("failed to save filter config: %v", err)
		http.Error(w, fmt.Sprintf(translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save filter: %v"), err), http.StatusInternalServerError)
		return
	}

	// ensure proper file extension for redirect
	if !strings.HasSuffix(filePath, ".filter") {
		filePath = filePath + ".filter"
	}

	// create success response
	successData := map[string]interface{}{
		"status":   "success",
		"message":  "filter saved successfully",
		"filePath": filePath,
	}

	successHTML := fmt.Sprintf(`<div class="success">filter saved successfully!</div>
		<script>setTimeout(() => window.location.href = '/files/%s', 1000);</script>`, filePath)

	writeResponse(w, r, successData, successHTML)
}

// @Summary Get filter form
// @Description Get HTML for complete filter form
// @Tags filter
// @Accept application/x-www-form-urlencoded
// @Param config formData string false "Filter configuration JSON"
// @Produce text/html
// @Success 200 {string} string "filter form html"
// @Router /api/filter/form [get]
func handleAPIGetFilterForm(w http.ResponseWriter, r *http.Request) {
	configStr := r.URL.Query().Get("config")
	var config *filter.Config

	if configStr != "" {
		config = &filter.Config{}
		if err := json.Unmarshal([]byte(configStr), config); err != nil {
			logging.LogError("failed to parse filter config: %v", err)
		}
	}

	html := render.RenderFilterForm(config)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get filter value input
// @Description Get HTML for filter value input based on metadata field type
// @Tags filter
// @Accept application/x-www-form-urlencoded
// @Param metadata formData string true "Metadata field name"
// @Param row_index formData int true "Row index"
// @Param value formData string false "Current value"
// @Produce text/html
// @Success 200 {string} string "filter value input html"
// @Router /api/filter/value-input [get]
func handleAPIGetFilterValueInput(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	rowIndexStr := r.FormValue("row_index")
	rowIndex, err := strconv.Atoi(rowIndexStr)
	if err != nil {
		rowIndex = 0
	}

	// get metadata from indexed field name
	metadataFieldName := fmt.Sprintf("metadata[%d]", rowIndex)
	metadata := r.FormValue(metadataFieldName)
	value := r.FormValue("value")

	logging.LogDebug("filter value input: rowIndex=%d, metadata=%s", rowIndex, metadata)

	inputId := fmt.Sprintf("filter-value-%d", rowIndex)
	inputName := fmt.Sprintf("value[%d]", rowIndex)

	html := render.RenderFilterValueInput(inputId, inputName, value, metadata)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Add filter criteria
// @Description Add new filter criteria row for filter forms
// @Tags filter
// @Accept application/x-www-form-urlencoded
// @Produce text/html
// @Success 200 {string} string "filter criteria row html"
// @Router /api/filter/add-criteria [post]
func handleAPIAddFilterCriteria(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	// generate unique criteria index based on timestamp
	criteriaIndex := int(time.Now().Unix()) % 1000

	html := render.RenderFilterCriteriaRow(criteriaIndex, nil)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
