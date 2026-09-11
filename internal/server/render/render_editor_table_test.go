package render

import (
	"strconv"
	"strings"
	"testing"

	"knov/internal/configmanager"
)

// withBoolSetting sets s to value for the duration of the test, restoring its
// prior value afterwards so global setting state doesn't leak between tests.
func withBoolSetting(t *testing.T, s *configmanager.BoolSetting, value bool) {
	t.Helper()
	original := s.Get()
	if err := s.SetFromString(strconv.FormatBool(value)); err != nil {
		t.Fatalf("failed to set %s: %v", s.Key(), err)
	}
	t.Cleanup(func() {
		if err := s.SetFromString(strconv.FormatBool(original)); err != nil {
			t.Fatalf("failed to restore %s: %v", s.Key(), err)
		}
	})
}

// TestTableOptionsJS_FieldsMapToTheirOwnSetting guards against tableOptionsJS silently
// pairing a setting with the wrong JS field name - e.g. sorting and editableColumns swapping -
// which previously would have been an easy-to-miss positional Sprintf argument reorder.
func TestTableOptionsJS_FieldsMapToTheirOwnSetting(t *testing.T) {
	settings := []struct {
		field   string
		setting *configmanager.BoolSetting
	}{
		{"selectableRows", configmanager.TableEditorSelectableRows},
		{"selectableCellRange", configmanager.TableEditorSelectableCellRange},
		{"pagination", configmanager.TableEditorPagination},
		{"rowNumbers", configmanager.TableEditorRowNumbers},
		{"sorting", configmanager.TableEditorSorting},
		{"editableColumns", configmanager.TableEditorEditableColumns},
		{"contextMenus", configmanager.TableEditorContextMenus},
	}

	for _, tc := range settings {
		t.Run(tc.field, func(t *testing.T) {
			// flip exactly one setting on, all others off, then confirm each JS field
			// carries the value of the setting it's supposed to represent.
			for _, s := range settings {
				withBoolSetting(t, s.setting, s.setting == tc.setting)
			}

			got := tableOptionsJS()

			for _, s := range settings {
				want := s.field + ": false"
				if s.setting == tc.setting {
					want = s.field + ": true"
				}
				if !strings.Contains(got, want) {
					t.Errorf("expected %q in output, got:\n%s", want, got)
				}
			}
		})
	}
}

// TestHeaderContextMenuScript_LabelsMapToTheirOwnAction guards against a translated label
// ending up paired with the wrong menu action (e.g. "insert column left" wired to the
// right-insert action) if the argument order is ever reshuffled.
func TestHeaderContextMenuScript_LabelsMapToTheirOwnAction(t *testing.T) {
	got := headerContextMenuScript("en")

	pairs := []struct{ label, action string }{
		{"insert column left", "insertColumn(column, true)"},
		{"insert column right", "insertColumn(column, false)"},
		{"align left", "setColumnAlign(column, 'left')"},
		{"align center", "setColumnAlign(column, 'center')"},
		{"align right", "setColumnAlign(column, 'right')"},
		{"remove column", "column.delete()"},
	}
	for _, p := range pairs {
		want := `label: "` + p.label + `", action: function(e, column) { ` + p.action
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}

// TestRowContextMenuScript_LabelsMapToTheirOwnAction mirrors
// TestHeaderContextMenuScript_LabelsMapToTheirOwnAction for the row context menu.
func TestRowContextMenuScript_LabelsMapToTheirOwnAction(t *testing.T) {
	got := rowContextMenuScript("en")

	pairs := []struct{ label, action string }{
		{"insert row above", "table.addRow(emptyRowData(), true, row)"},
		{"insert row below", "table.addRow(emptyRowData(), false, row)"},
		{"remove row", "row.delete()"},
	}
	for _, p := range pairs {
		want := `label: "` + p.label + `", action: function(e, row) { ` + p.action
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}
