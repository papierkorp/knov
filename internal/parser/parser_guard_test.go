package parser

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// guardRule is a way of reading or writing a link path by hand. allowed is how many matches each
// repo-relative file may have (with the reason as comment), skip the folders the rule doesn't
// apply to.
type guardRule struct {
	name    string
	re      *regexp.Regexp
	exts    []string
	skip    []string
	allowed map[string]int
}

var guardRules = []guardRule{
	{
		name: "url path escaping outside the codec and pathutils",
		re:   regexp.MustCompile(`url\.Path(?:Unescape|Escape)\(`),
		exts: []string{".go"},
		allowed: map[string]int{
			"internal/parser/link_rewrite.go": 2, // decodeLinkPath, AnchorText
			"internal/pathutils/pathutils.go": 1, // escapeRelPath, the To*URL helpers
		},
	},
	{
		name: `link regex (\]\( or \[\[) outside internal/parser`,
		re:   regexp.MustCompile(`\\\]\\\(|\\\[\\\[`),
		exts: []string{".go", ".js", ".gohtml"},
		skip: []string{"internal/parser/"},
		allowed: map[string]int{
			"internal/dokuwikiconverter/converter_process.go": 1, // dokuwiki [[...]] syntax, not markdown
			"static/wiki-autocomplete.js":                     2, // an open [[ / ]( before the cursor starts autocomplete
		},
	},
	{
		name: "hand-built markdown link",
		re:   regexp.MustCompile(`"\]\(" \+|\[%s\]\(%s\)`),
		exts: []string{".go"},
		allowed: map[string]int{
			"internal/parser/link_rewrite.go":    1, // Link.String
			"internal/parser/parser_markdown.go": 4, // RenderLinks writes app urls
		},
	},
	{
		name: "converting a string to pathutils.MetaPath / DocsRel (use DocsPath, MediaPath, ParseMeta, NewDocsRel)",
		re:   regexp.MustCompile(`pathutils\.(?:MetaPath|DocsRel)\(`),
		exts: []string{".go"},
	},
	{
		name: "path guessing (ToRelative / ToWithPrefix / ToDocsPath / ToFullPath / GuessMeta) - take a MetaPath / DocsRel and build it with DocsPath, MediaPath, ParseMeta or FromFullPath",
		re:   regexp.MustCompile(`pathutils\.(?:ToRelative|ToWithPrefix|ToDocsPath|ToFullPath|GuessMeta)\(`),
		exts: []string{".go"},
		allowed: map[string]int{
			// the guesses that are left: each one is a place a typed path is not threaded through yet, the
			// list only shrinks
			"internal/book/book.go":                              3,
			"internal/configeditor/configeditor.go":              6,
			"internal/contentHandler/contenthandler_markdown.go": 5,
			"internal/dokuwikiconverter/converter.go":            1,
			"internal/files/metadata_links.go":                   2,
			"internal/files/metadata_purge.go":                   1,
			"internal/files/reserved_migration.go":               1,
			"internal/filter/filter.go":                          2,
			"internal/git/git.go":                                12,
			"internal/job/asyncdelete.go":                        2,
			"internal/job/cronjob.go":                            4,
			"internal/job/manualjob.go":                          5,
			"internal/kanban/issues.go":                          1,
			"internal/parser/parser_markdown.go":                 5,
			"internal/parser/table.go":                           1,
			"internal/searchStorage/searchStorage_sqlite.go":     1,
			"internal/server/api_chat.go":                        6,
			"internal/server/api_editor.go":                      15,
			"internal/server/api_files.go":                       23,
			"internal/server/api_files_versions.go":              3,
			"internal/server/api_git.go":                         1,
			"internal/server/api_links.go":                       2,
			"internal/server/api_media.go":                       2,
			"internal/server/api_metadata.go":                    5,
			"internal/server/pages_files.go":                     2,
			"internal/server/render/render_chat.go":              1,
			"internal/server/render/render_editor_codemirror.go": 6,
			"internal/server/render/render_editor_entry.go":      4,
			"internal/server/render/render_editor_filter.go":     1,
			"internal/server/render/render_editor_listtodo.go":   4,
			"internal/server/render/render_editor_table.go":      2,
			"internal/server/render/render_editor_tracker.go":    3,
			"internal/server/render/render_files.go":             4,
			"internal/server/render/render_git.go":               6,
			"internal/server/render/render_links.go":             1,
			"internal/server/render/render_metadata.go":          1,
			"internal/server/render/render_search.go":            2,
			"internal/thememanager/template_data.go":             1,
		},
	},
	{
		name: "hand-rolled %20 encoding",
		re:   regexp.MustCompile(`(?i)replace(?:all)?\([^)]*%20`),
		exts: []string{".go", ".js", ".gohtml"},
	},
	{
		name: "encodeURIComponent (link paths go through the server, pathutils)",
		re:   regexp.MustCompile(`encodeURIComponent\b`),
		exts: []string{".go", ".js", ".gohtml"},
		allowed: map[string]int{
			// query parameter values
			"internal/server/render/render_system.go": 2,
			"static/wiki-autocomplete.js":             5,
			"themes/builtin/js/history-search.js":     3,
			"themes/builtin/js/kanban.js":             3,
			"themes/builtin/js/panel-content.js":      4,
			"themes/builtin/js/panel-file.js":         4,
			// pathURL, the js side of pathutils.ToRouteURL
			"themes/builtin/js/rail-core.js": 1,
		},
	},
	{
		name: "decodeURIComponent (link paths are decoded by the server)",
		re:   regexp.MustCompile(`decodeURIComponent\b`),
		exts: []string{".go", ".js", ".gohtml"},
		allowed: map[string]int{
			"themes/builtin/js/panel-content.js": 1, // a url fragment
			"themes/builtin/js/panel-file.js":    3, // the displayed name and the page path, from location.pathname
		},
	},
	{
		name: "goldmark render without RenderLinks first",
		re:   regexp.MustCompile(`goldmark\.New\(|\.Convert\(|RenderInlineMarkdown\(`),
		exts: []string{".go"},
		allowed: map[string]int{
			// Render (after Parse ran RenderLinks), RenderInlineMarkdown / RenderHeadingInline (heading
			// and <summary> text, whose links RenderLinks already rewrote)
			"internal/parser/parser_markdown.go": 7,
			"internal/pdfexport/pdfexport.go":    1, // runs RenderLinks first
			"internal/server/api_tables.go":      2, // table cells, after RenderLinks
		},
	},
}

// isCommentLine reports whether a source line is only a comment.
func isCommentLine(line string) bool {
	line = strings.TrimSpace(line)
	return slices.ContainsFunc([]string{"//", "/*", "* ", "<!--", "{{/*"}, func(p string) bool { return strings.HasPrefix(line, p) })
}

// a link path is decoded and encoded only by the codec (link_rewrite.go) and turned into an url
// only by pathutils - every other place reading or writing one by hand has to be allowed in
// guardRules, so a new hand-rolled link reader or writer fails the build instead of relying on
// review. tests, the test suites (they write links like a user types them) and vendored *.min.js
// are not scanned.
func TestNoHandRolledLinkHandling(t *testing.T) {
	root := filepath.Join("..", "..")
	counts := make([]map[string]int, len(guardRules))
	for i := range counts {
		counts[i] = map[string]int{}
	}
	for _, dir := range []string{"internal", "static", "themes"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if strings.HasSuffix(rel, "_test.go") || strings.HasSuffix(rel, ".min.js") || strings.HasPrefix(rel, "internal/test/") {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for i, r := range guardRules {
				if !slices.Contains(r.exts, filepath.Ext(rel)) || slices.ContainsFunc(r.skip, func(s string) bool { return strings.HasPrefix(rel, s) }) {
					continue
				}
				for _, line := range strings.Split(string(data), "\n") {
					if !isCommentLine(line) {
						counts[i][rel] += len(r.re.FindAllStringIndex(line, -1))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, r := range guardRules {
		for file, n := range counts[i] {
			if n > r.allowed[file] {
				t.Errorf("%s: %s in %s (%d, allowed %d) - use parser.Link / LinkTarget / RenderLinks and the pathutils url helpers, or allow it in guardRules with the reason", r.name, r.re, file, n, r.allowed[file])
			}
		}
		for file, n := range r.allowed {
			if counts[i][file] < n {
				t.Errorf("%s: %s allows %d in %s, found %d - lower the allow-list", r.name, r.re, n, file, counts[i][file])
			}
		}
	}
	if t.Failed() {
		fmt.Println("lines are counted outside comments, see isCommentLine")
	}
}
