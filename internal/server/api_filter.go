// Package server - Filter API handlers
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/server/render"
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
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	config := parseFilterConfigFromForm(r)
	if err := filter.ValidateConfig(config); err != nil {
		logging.LogError("invalid filter config: %v", err)
		http.Error(w, fmt.Sprintf("invalid filter config: %v", err), http.StatusBadRequest)
		return
	}

	logging.LogDebug("built filter config: %+v", config)

	result, err := filter.FilterFilesWithConfig(config)
	if err != nil {
		logging.LogError("failed to filter files: %v", err)
		http.Error(w, "failed to filter files", http.StatusInternalServerError)
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
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	metadata := r.FormValue("metadata")
	rowIndexStr := r.FormValue("row_index")
	value := r.FormValue("value")

	rowIndex, err := strconv.Atoi(rowIndexStr)
	if err != nil {
		rowIndex = 0
	}

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
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	// generate unique criteria index based on timestamp
	criteriaIndex := int(time.Now().Unix()) % 1000

	html := render.RenderFilterCriteriaRow(criteriaIndex, nil)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func parseFilterConfigFromForm(r *http.Request) *filter.Config {
	logic := r.FormValue("logic")
	if logic == "" {
		logic = "and"
	}

	display := r.FormValue("display")
	if display == "" {
		display = "list"
	}

	limitStr := r.FormValue("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	var criteria []filter.Criteria
	metadata := r.Form["metadata[]"]
	if len(metadata) == 0 {
		// try alternative form encoding
		for i := 0; i < 50; i++ { // safety limit
			meta := r.FormValue(fmt.Sprintf("metadata[%d]", i))
			if meta == "" {
				break
			}
			metadata = append(metadata, meta)
		}
	}

	operators := r.Form["operator[]"]
	if len(operators) == 0 {
		for i := 0; i < len(metadata); i++ {
			op := r.FormValue(fmt.Sprintf("operator[%d]", i))
			if op == "" {
				op = "equals"
			}
			operators = append(operators, op)
		}
	}

	values := r.Form["value[]"]
	if len(values) == 0 {
		for i := 0; i < len(metadata); i++ {
			val := r.FormValue(fmt.Sprintf("value[%d]", i))
			values = append(values, val)
		}
	}

	actions := r.Form["action[]"]
	if len(actions) == 0 {
		for i := 0; i < len(metadata); i++ {
			act := r.FormValue(fmt.Sprintf("action[%d]", i))
			if act == "" {
				act = "include"
			}
			actions = append(actions, act)
		}
	}

	maxLen := len(metadata)
	for i := 0; i < maxLen; i++ {
		if i < len(operators) && i < len(values) && metadata[i] != "" && operators[i] != "" && values[i] != "" {
			action := "include"
			if i < len(actions) && actions[i] != "" {
				action = actions[i]
			}

			criteria = append(criteria, filter.Criteria{
				Metadata: metadata[i],
				Operator: operators[i],
				Value:    values[i],
				Action:   action,
			})
		}
	}

	return &filter.Config{
		Criteria: criteria,
		Logic:    logic,
		Display:  display,
		Limit:    limit,
	}
}
