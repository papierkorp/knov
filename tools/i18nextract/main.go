// Command i18nextract scans the codebase for translatable strings and refreshes the
// translation catalog. It replaces the old grep/sed/jq based static/generate-translations.sh
// with real Go parsing (go/parser), which lets it find translatable text that plain regex
// extraction can't: string literals assigned to a Label, Desc or Description struct field
// (e.g. settings and nav definitions) are picked up too, not just literal arguments passed
// directly to a translation call.
//
// It finds four kinds of translatable text:
//   - translation.Sprintf("literal", ...) and translation.SprintfForRequest(lang, "literal", ...)
//     calls anywhere in the Go source
//   - bare t("literal", ...) calls - this codebase's pattern for threading a per-request
//     language into the render package without a lang parameter on every function
//   - {{T "literal"}} calls in .gohtml templates
//   - string literals assigned to a Label, Desc or Description struct field, wherever that
//     value later reaches a translation call through a variable instead of a literal (which the
//     call-based rules above can't see)
//
// Run via `make translation` (or `go generate` from internal/translation). Requires the gotext
// tool (`go install golang.org/x/text/cmd/gotext@latest`) to regenerate catalog.go from the
// extracted strings.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// repoRoot is relative to the working directory go generate runs this in - internal/translation.
const repoRoot = "../.."

var supportedLanguages = []string{"en", "de"}

var skipDirs = map[string]bool{
	".git": true, "bin": true, "tempai": true, "node_modules": true,
	"data": true, "data2": true, "data3": true, "storage": true, "backups": true, "logs": true,
}

var labelFields = map[string]bool{"Label": true, "Desc": true, "Description": true}

var gohtmlTPattern = regexp.MustCompile(`\{\{\s*T\s+"((?:[^"\\]|\\.)*)"\s*\}\}`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "i18nextract:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("extracting translatable strings from .gohtml and .go files...")
	strs, err := extractStrings()
	if err != nil {
		return err
	}
	fmt.Printf("%d translation strings found\n", len(strs))

	const tempFile = "temp_extracted.go"
	if err := writeExtractionStub(tempFile, strs); err != nil {
		return err
	}
	fmt.Println("generated temporary extraction file:", tempFile)
	defer os.Remove(tempFile)

	fmt.Println("updating translation catalog...")
	cmd := exec.Command("gotext", "-srclang=en", "update", "-out=catalog.go",
		"-lang="+strings.Join(supportedLanguages, ","), ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gotext update failed (is it installed? go install golang.org/x/text/cmd/gotext@latest): %w", err)
	}

	fmt.Println("syncing translation files...")
	for _, lang := range supportedLanguages {
		fmt.Println("syncing translation file: locales/" + lang)
		if err := syncMessages(lang); err != nil {
			return err
		}
		fmt.Println("successfully synced locales/" + lang + "/messages.gotext.json")
	}

	fmt.Println("translation catalog updated successfully!")
	fmt.Println("cleaned up temporary file")
	fmt.Println("translation extraction complete!")
	return nil
}

// extractStrings walks the repo and collects every translatable string it can find, deduped
// and sorted for stable output.
func extractStrings() ([]string, error) {
	set := map[string]bool{}

	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".gohtml"):
			return extractGohtml(path, set)
		case strings.HasSuffix(path, ".go"):
			return extractGo(path, set)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

func isGeneratedGoFile(path string) bool {
	slash := filepath.ToSlash(path)
	base := filepath.Base(path)
	if base == "catalog.go" || base == "temp_extracted.go" {
		return true
	}
	return base == "docs.go" && strings.Contains(slash, "/server/swagger/")
}

func extractGo(path string, set map[string]bool) error {
	if isGeneratedGoFile(path) {
		return nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	// test fixtures often set Label/Desc/Description on synthetic data with no i18n intent
	// (e.g. files.Reference.Description in sample data) - only apply that rule to real source
	allowLabelFields := !strings.Contains(filepath.ToSlash(path), "/internal/test/")

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			extractCall(node, set)
		case *ast.KeyValueExpr:
			if allowLabelFields {
				extractLabelField(node, set)
			}
		}
		return true
	})
	return nil
}

// extractCall picks up translation.Sprintf("literal", ...) and
// translation.SprintfForRequest(lang, "literal", ...) calls, plus bare t("literal", ...) calls -
// this codebase's established pattern (in render_system.go, render_backup.go, render_themes.go,
// render_notifications.go, render_editor_codemirror.go, render_editor_shared.go,
// render_testdata.go, api_settings.go) for threading a per-request language into the render
// package without a lang parameter on every function: t := func(key string, args ...any) string
// { return translation.SprintfForRequest(lang, key, args...) }. No other callable named exactly
// "t" exists anywhere in the codebase (every other "t :=" is a string loop variable, which can't
// be called), so this is unambiguous.
func extractCall(call *ast.CallExpr, set map[string]bool) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		pkg, ok := fun.X.(*ast.Ident)
		if !ok || pkg.Name != "translation" {
			return
		}
		var argIdx int
		switch fun.Sel.Name {
		case "Sprintf":
			argIdx = 0
		case "SprintfForRequest":
			argIdx = 1
		default:
			return
		}
		if len(call.Args) <= argIdx {
			return
		}
		if s, ok := stringLit(call.Args[argIdx]); ok {
			set[s] = true
		}
	case *ast.Ident:
		if fun.Name != "t" || len(call.Args) == 0 {
			return
		}
		if s, ok := stringLit(call.Args[0]); ok {
			set[s] = true
		}
	}
}

// extractLabelField picks up string literals assigned to a Label, Desc or Description field in
// a struct literal, e.g. &StringSetting{Label: "Language", Desc: "choose your preferred..."}.
func extractLabelField(kv *ast.KeyValueExpr, set map[string]bool) {
	ident, ok := kv.Key.(*ast.Ident)
	if !ok || !labelFields[ident.Name] {
		return
	}
	if s, ok := stringLit(kv.Value); ok && s != "" {
		set[s] = true
	}
}

func stringLit(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func extractGohtml(path string, set map[string]bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, m := range gohtmlTPattern.FindAllStringSubmatch(string(data), -1) {
		if s, err := strconv.Unquote(`"` + m[1] + `"`); err == nil {
			set[s] = true
		}
	}
	return nil
}

// writeExtractionStub writes a throwaway Go file with one fake message.Printer.Sprintf call
// per extracted string, so `gotext update`'s own real static analysis - which only understands
// literal args to *message.Printer methods - has something to find.
func writeExtractionStub(path string, strs []string) error {
	var b strings.Builder
	b.WriteString("// temporary file for gotext extraction - auto-generated, safe to delete\n")
	b.WriteString("package translation\n\n")
	b.WriteString("import \"golang.org/x/text/message\"\n\n")
	b.WriteString("func init() {\n")
	b.WriteString("\tp := message.NewPrinter(message.MatchLanguage(\"en\"))\n")
	for _, s := range strs {
		fmt.Fprintf(&b, "\t_ = p.Sprintf(%s)\n", strconv.Quote(s))
	}
	if len(strs) == 0 {
		b.WriteString("\t_ = p // avoid unused variable error\n")
	}
	b.WriteString("}\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

type gotextMessage struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	Translation string `json:"translation"`
}

type gotextFile struct {
	Language string          `json:"language"`
	Messages []gotextMessage `json:"messages"`
}

// syncMessages merges gotext's fresh extraction output (out.gotext.json) into the persisted
// translation source of truth (messages.gotext.json), keeping existing translations and
// dropping entries for strings that no longer exist in the source.
func syncMessages(lang string) error {
	dir := filepath.Join("locales", lang)
	outPath := filepath.Join(dir, "out.gotext.json")
	messagesPath := filepath.Join(dir, "messages.gotext.json")

	outData, err := os.ReadFile(outPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", outPath, err)
	}
	var out gotextFile
	if err := json.Unmarshal(outData, &out); err != nil {
		return fmt.Errorf("parse %s: %w", outPath, err)
	}

	existing := map[string]string{}
	prevData, err := os.ReadFile(messagesPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// no prior translations yet
	case err != nil:
		return fmt.Errorf("read %s: %w", messagesPath, err)
	default:
		var cur gotextFile
		if err := json.Unmarshal(prevData, &cur); err != nil {
			return fmt.Errorf("parse %s (refusing to overwrite existing translations): %w", messagesPath, err)
		}
		for _, m := range cur.Messages {
			if m.Translation != "" {
				existing[m.ID] = m.Translation
			}
		}
	}

	merged := gotextFile{Language: lang, Messages: make([]gotextMessage, len(out.Messages))}
	for i, m := range out.Messages {
		merged.Messages[i] = gotextMessage{ID: m.ID, Message: m.Message, Translation: existing[m.ID]}
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	backupPath := messagesPath + ".backup"
	if len(prevData) > 0 {
		if err := os.WriteFile(backupPath, prevData, 0o644); err != nil {
			return fmt.Errorf("backup %s: %w", messagesPath, err)
		}
	}
	if err := os.WriteFile(messagesPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s (backup preserved at %s): %w", messagesPath, backupPath, err)
	}
	os.Remove(backupPath)
	return nil
}
