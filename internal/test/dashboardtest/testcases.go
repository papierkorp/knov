package dashboardtest

import (
	"encoding/json"
	"fmt"
	"strings"

	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/pathutils"
	"knov/internal/test"
	"knov/internal/utils"
)

func caseCreateDashboard() test.CaseResult {
	name := "create-dashboard"

	d := &dashboard.Dashboard{Name: "Dashtest Create", Layout: dashboard.OneColumn}
	if err := dashboard.Create(d); err != nil {
		return errCase(name, err)
	}
	defer dashboard.Delete(d.ID)

	got, err := dashboard.Get(d.ID)
	success := err == nil && got != nil && got.Name == "Dashtest Create" && d.ID == utils.CleanseID("Dashtest Create")

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("id=%q retrievable with matching name", utils.CleanseID("Dashtest Create")),
		Success:  success,
	}
	if got != nil {
		cr.Actual = fmt.Sprintf("id=%q name=%q", d.ID, got.Name)
	}
	if !success {
		cr.Error = "created dashboard not retrievable or id/name mismatch"
	}
	return cr
}

func caseGetAllDashboards() test.CaseResult {
	name := "get-all-dashboards"

	a := &dashboard.Dashboard{Name: "Dashtest GetAll A", Layout: dashboard.OneColumn}
	b := &dashboard.Dashboard{Name: "Dashtest GetAll B", Layout: dashboard.TwoColumns}
	if err := dashboard.Create(a); err != nil {
		return errCase(name, err)
	}
	defer dashboard.Delete(a.ID)
	if err := dashboard.Create(b); err != nil {
		return errCase(name, err)
	}
	defer dashboard.Delete(b.ID)

	all, err := dashboard.GetAll()
	if err != nil {
		return errCase(name, err)
	}

	foundA, foundB := false, false
	for _, d := range all {
		if d.ID == a.ID {
			foundA = true
		}
		if d.ID == b.ID {
			foundB = true
		}
	}

	success := foundA && foundB
	cr := test.CaseResult{
		Name:     name,
		Expected: "GetAll includes both newly created dashboards",
		Actual:   fmt.Sprintf("total=%d foundA=%v foundB=%v", len(all), foundA, foundB),
		Success:  success,
	}
	if !success {
		cr.Error = "GetAll did not include one or both newly created dashboards"
	}
	return cr
}

func caseUpdateDashboard() test.CaseResult {
	name := "update-dashboard"

	d := &dashboard.Dashboard{Name: "Dashtest Update", Layout: dashboard.OneColumn}
	if err := dashboard.Create(d); err != nil {
		return errCase(name, err)
	}
	defer dashboard.Delete(d.ID)

	d.Layout = dashboard.TwoColumns
	if err := dashboard.Update(d); err != nil {
		return errCase(name, err)
	}

	got, err := dashboard.Get(d.ID)
	success := err == nil && got != nil && got.Layout == dashboard.TwoColumns

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("layout=%q after update", dashboard.TwoColumns),
		Success:  success,
	}
	if got != nil {
		cr.Actual = fmt.Sprintf("layout=%q", got.Layout)
	}
	if !success {
		cr.Error = "dashboard layout not updated"
	}
	return cr
}

// caseRenameDashboard mirrors handleAPIRenameDashboard: fetch, overwrite Name, Update.
func caseRenameDashboard() test.CaseResult {
	name := "rename-dashboard"

	d := &dashboard.Dashboard{Name: "Dashtest Rename", Layout: dashboard.OneColumn}
	if err := dashboard.Create(d); err != nil {
		return errCase(name, err)
	}
	defer dashboard.Delete(d.ID)
	originalID := d.ID

	got, err := dashboard.Get(originalID)
	if err != nil {
		return errCase(name, err)
	}
	got.Name = "Dashtest Renamed"
	if err := dashboard.Update(got); err != nil {
		return errCase(name, err)
	}

	after, err := dashboard.Get(originalID)
	success := err == nil && after != nil && after.Name == "Dashtest Renamed" && after.ID == originalID

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("id unchanged (%q), name=\"Dashtest Renamed\"", originalID),
		Success:  success,
	}
	if after != nil {
		cr.Actual = fmt.Sprintf("id=%q name=%q", after.ID, after.Name)
	}
	if !success {
		cr.Error = "rename did not update name in place or changed the id"
	}
	return cr
}

func caseDeleteDashboard() test.CaseResult {
	name := "delete-dashboard"

	d := &dashboard.Dashboard{Name: "Dashtest Delete", Layout: dashboard.OneColumn}
	if err := dashboard.Create(d); err != nil {
		return errCase(name, err)
	}

	if err := dashboard.Delete(d.ID); err != nil {
		return errCase(name, err)
	}

	_, err := dashboard.Get(d.ID)
	success := err != nil

	cr := test.CaseResult{
		Name:     name,
		Expected: "Get errors after delete",
		Success:  success,
	}
	if !success {
		cr.Actual = "dashboard still retrievable"
		cr.Error = "deleted dashboard still retrievable"
	} else {
		cr.Actual = fmt.Sprintf("error=%q", err.Error())
	}
	return cr
}

// caseExportImportDashboard mirrors handleAPIExportDashboard (json.MarshalIndent) followed
// by handleAPIImportDashboard (decode, reset ID, Create) - both are one-liners around
// dashboard.Get/Create in internal/server, trivial to replicate without importing it.
func caseExportImportDashboard() test.CaseResult {
	name := "export-import-dashboard"

	original := &dashboard.Dashboard{
		Name:   "Dashtest Export",
		Layout: dashboard.OneColumn,
		Widgets: []dashboard.Widget{
			{Type: dashboard.WidgetTypeStatic, Title: "note", Config: dashboard.WidgetConfig{
				Static: &dashboard.StaticConfig{Content: "hello", Format: "text"},
			}},
		},
	}
	if err := dashboard.Create(original); err != nil {
		return errCase(name, err)
	}
	defer dashboard.Delete(original.ID)

	fetched, err := dashboard.Get(original.ID)
	if err != nil {
		return errCase(name, err)
	}

	exported, err := json.MarshalIndent(fetched, "", "  ")
	if err != nil {
		return errCase(name, err)
	}

	var imported dashboard.Dashboard
	if err := json.Unmarshal(exported, &imported); err != nil {
		return errCase(name, err)
	}
	// mirrors the handler's optional name override on import: Create derives id from Name,
	// so importing under the exact same name as the still-existing original would collide -
	// same as the real "import" form requires a distinct name when the original still exists.
	imported.ID = ""
	imported.Name = "Dashtest Export Imported"
	if err := dashboard.Create(&imported); err != nil {
		return errCase(name, err)
	}
	defer dashboard.Delete(imported.ID)

	success := imported.ID != original.ID && imported.Layout == original.Layout &&
		len(imported.Widgets) == len(original.Widgets) &&
		imported.Widgets[0].Config.Static.Content == original.Widgets[0].Config.Static.Content

	cr := test.CaseResult{
		Name:     name,
		Expected: "imported copy gets a fresh id but matching layout/widget content",
		Actual:   fmt.Sprintf("originalID=%q importedID=%q layout=%q widgets=%d", original.ID, imported.ID, imported.Layout, len(imported.Widgets)),
		Success:  success,
	}
	if !success {
		cr.Error = "export/import round-trip did not produce the expected dashboard"
	}
	return cr
}

// caseWidgetFilterData covers the filter widget's underlying data resolution (the render
// dispatch itself lives in internal/server/render, unreachable here - see package doc).
func caseWidgetFilterData() test.CaseResult {
	name := "widget-filter-data"

	cfg := &filter.Config{
		Criteria: []filter.Criteria{{Metadata: "tags", Operator: "contains", Value: sampleTag, Action: "include"}},
		Logic:    "and",
	}
	result, err := filter.FilterFilesWithConfig(cfg)
	if err != nil {
		return errCase(name, err)
	}

	found := false
	for _, f := range result.Files {
		if f.Name == sampleFile {
			found = true
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("filter widget data includes %s", sampleFile),
		Actual:   fmt.Sprintf("%d files matched", len(result.Files)),
		Success:  found,
	}
	if !found {
		cr.Error = "filter widget's underlying FilterFilesWithConfig call did not return the sample file"
	}
	return cr
}

// caseWidgetFileContentData covers the fileContent widget's underlying files.GetFileContent call.
func caseWidgetFileContentData() test.CaseResult {
	name := "widget-file-content-data"

	content, err := files.GetFileContent(pathutils.ToDocsPath(testPath(sampleFile)))
	if err != nil {
		return errCase(name, err)
	}

	success := content != nil && strings.Contains(content.HTML, sampleContentMarker)
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("rendered HTML contains %q", sampleContentMarker),
		Success:  success,
	}
	if content != nil {
		cr.Actual = fmt.Sprintf("html=%q", content.HTML)
	}
	if !success {
		cr.Error = "fileContent widget's underlying GetFileContent call did not return expected content"
	}
	return cr
}

// caseWidgetAggregateData covers the tags/collections/folders widgets' underlying calls -
// these are pass-through cache reads with no widget-specific logic to break beyond "errors".
func caseWidgetAggregateData() test.CaseResult {
	name := "widget-aggregate-data"

	if _, err := files.GetAllTagsCountFromCache(); err != nil {
		return errCase(name, err)
	}
	if _, err := files.GetAllCollectionsCountFromCache(); err != nil {
		return errCase(name, err)
	}
	if _, err := files.GetAllFoldersCountFromCache(); err != nil {
		return errCase(name, err)
	}

	return test.CaseResult{
		Name:     name,
		Expected: "tags/collections/folders widget data calls succeed",
		Actual:   "no error",
		Success:  true,
	}
}
