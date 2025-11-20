// Package server - Filter API handlers
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

	// Extract all indexed form values
	formData := make(map[int]map[string]string)

	for key, values := range r.Form {
		if len(values) == 0 {
			continue
		}

		var field string
		var index int
		var err error

		if strings.HasPrefix(key, "metadata[") && strings.HasSuffix(key, "]") {
			field = "metadata"
			indexStr := key[9 : len(key)-1] // extract index from metadata[index]
			index, err = strconv.Atoi(indexStr)
		} else if strings.HasPrefix(key, "operator[") && strings.HasSuffix(key, "]") {
			field = "operator"
			indexStr := key[9 : len(key)-1] // extract index from operator[index]
			index, err = strconv.Atoi(indexStr)
		} else if strings.HasPrefix(key, "value[") && strings.HasSuffix(key, "]") {
			field = "value"
			indexStr := key[6 : len(key)-1] // extract index from value[index]
			index, err = strconv.Atoi(indexStr)
		} else if strings.HasPrefix(key, "action[") && strings.HasSuffix(key, "]") {
			field = "action"
			indexStr := key[7 : len(key)-1] // extract index from action[index]
			index, err = strconv.Atoi(indexStr)
		} else {
			continue
		}

		if err != nil {
			continue
		}

		if formData[index] == nil {
			formData[index] = make(map[string]string)
		}
		formData[index][field] = values[0]
	}

	// fallback to array form if no indexed data found
	if len(formData) == 0 {
		metadata := r.Form["metadata[]"]
		operators := r.Form["operator[]"]
		values := r.Form["value[]"]
		actions := r.Form["action[]"]

		maxLen := len(metadata)
		if len(operators) > maxLen {
			maxLen = len(operators)
		}
		if len(values) > maxLen {
			maxLen = len(values)
		}
		if len(actions) > maxLen {
			maxLen = len(actions)
		}

		for i := 0; i < maxLen; i++ {
			if formData[i] == nil {
				formData[i] = make(map[string]string)
			}

			if i < len(metadata) {
				formData[i]["metadata"] = metadata[i]
			}
			if i < len(operators) {
				formData[i]["operator"] = operators[i]
			} else {
				formData[i]["operator"] = "equals"
			}
			if i < len(values) {
				formData[i]["value"] = values[i]
			}
			if i < len(actions) {
				formData[i]["action"] = actions[i]
			} else {
				formData[i]["action"] = "include"
			}
		}
	}

	// build criteria from grouped form data
	for _, data := range formData {
		metadata := data["metadata"]
		operator := data["operator"]
		value := data["value"]
		action := data["action"]

		if metadata != "" && operator != "" && value != "" {
			if operator == "" {
				operator = "equals"
			}
			if action == "" {
				action = "include"
			}

			criteria = append(criteria, filter.Criteria{
				Metadata: metadata,
				Operator: operator,
				Value:    value,
				Action:   action,
			})
		}
	}

	logging.LogDebug("parsed %d filter criteria", len(criteria))
	for i, c := range criteria {
		logging.LogDebug("criteria %d: %s %s %s (%s)", i, c.Metadata, c.Operator, c.Value, c.Action)
	}

	return &filter.Config{
		Criteria: criteria,
		Logic:    logic,
		Display:  display,
		Limit:    limit,
	}
}
