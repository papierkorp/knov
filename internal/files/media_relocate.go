package files

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/types"
)

// MisplacedMedia is a non-text or allowed-extension file found in the docs folder (e.g. copied
// in from another wiki). Target is its media-relative destination, mirroring its docs folder - empty when
// its type isn't an allowed media type, in which case it is only reported, never moved.
// DetectedAs is the mime type or extension it was detected by, DetectedBy "content" or "extension".
type MisplacedMedia struct {
	Path       string `json:"path"`
	Target     string `json:"target,omitempty"`
	DetectedAs string `json:"detectedAs"`
	DetectedBy string `json:"detectedBy"`
}

// MediaRelocateResult is the outcome of a RelocateMisplacedMedia run. Failed counts both files
// that couldn't be moved and docs whose links couldn't be written.
type MediaRelocateResult struct {
	Moved        int `json:"moved"`
	Failed       int `json:"failed"`
	FilesUpdated int `json:"filesUpdated"`
}

// ScanMisplacedMedia returns every misplaced file with its planned target, without changing anything.
func ScanMisplacedMedia() ([]MisplacedMedia, error) {
	items, _, err := listMisplacedMedia()
	return items, err
}

// RelocateMisplacedMedia moves the selected allowed-type media files from the docs folder into the
// media folder (mirroring its docs folder) and rewrites all links in markdown docs pointing
// to it - doc-relative, docs-root ("/folder/img.png", as wiki.js writes them), markdown, wiki and
// html src/href - to its /media/ URL. Non-allowed binaries are left in place.
func RelocateMisplacedMedia(key logging.Key, selected []string) (MediaRelocateResult, error) {
	items, paths, err := listMisplacedMedia()
	if err != nil {
		return MediaRelocateResult{}, err
	}

	// paths are docs-relative, so they get an explicit docs/ or media/ prefix before going
	// through pathutils - otherwise a docs folder named "media", "docs" or "files" is stripped
	var result MediaRelocateResult
	moved := make(map[string]string, len(items))
	for _, item := range items {
		if item.Target == "" || !slices.Contains(selected, item.Path) {
			continue
		}
		if err := moveDocsToMedia(pathutils.ToDocsPath("docs/"+item.Path), pathutils.ToMediaPath("media/"+item.Target)); err != nil {
			logging.LogError(key, "failed to move %s -> media/%s: %v", item.Path, item.Target, err)
			result.Failed++
			continue
		}
		if err := moveFileMetadata(key, "docs/"+item.Path, "media/"+item.Target); err != nil {
			logging.LogWarning(key, "failed to move metadata for %s: %v", item.Path, err)
		}
		logging.LogInfo(key, "moved %s -> media/%s", item.Path, item.Target)
		result.Moved++
		moved[item.Path] = item.Target
	}

	if result.Moved > 0 {
		updated, failed := relinkDocs(key, paths, moved)
		result.FilesUpdated = updated
		result.Failed += failed
		if failed > 0 {
			// the files are already moved, so a rerun won't find them again
			logging.LogWarning(key, "%d docs could not be relinked, fix their links via repair broken links", failed)
		}
		RefreshCaches()
	}
	return result, nil
}

// listMisplacedMedia returns the misplaced media in the docs folder (allowed media types get a
// conflict-free media target, other binaries none) and every docs file path.
func listMisplacedMedia() (items []MisplacedMedia, paths []string, err error) {
	paths, err = contentStorage.ListFiles()
	if err != nil {
		return nil, nil, err
	}
	items = []MisplacedMedia{} // an empty scan is [], not null

	planned := make(map[string]bool) // targets already handed out, so two files never get the same one
	for _, rel := range paths {
		if parser.IsMarkdownExtension(rel) {
			continue
		}
		// text files (.txt, .json, .xml, ...) render as docs, so only binaries and files whose
		// extension is listed in the allowed media types (e.g. .excalidraw) are moved - the content
		// is sniffed like on upload, so binaries with unknown extensions are caught too. svg sniffs
		// as text but is still an image
		ext := strings.ToLower(filepath.Ext(rel))
		mimeType := sniffMimeType(pathutils.ToDocsPath("docs/" + rel))
		item := MisplacedMedia{Path: rel, DetectedAs: mimeType, DetectedBy: "content"}
		if extMime := types.MimeTypeByExtension(ext); strings.HasPrefix(extMime, "image/") {
			mimeType = extMime
			item.DetectedAs, item.DetectedBy = extMime, "extension"
		}
		if mimeType == "" || strings.HasPrefix(mimeType, "text/") {
			if !configmanager.IsAllowedMediaExtension(rel) {
				continue
			}
			item.DetectedAs, item.DetectedBy = ext, "extension"
		}
		if configmanager.IsAllowedMediaType(rel, mimeType) {
			ext := path.Ext(rel)
			item.Target = rel
			for i := 1; planned[item.Target] || fileExists(pathutils.ToMediaPath("media/"+item.Target)); i++ {
				item.Target = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(rel, ext), i, ext)
			}
			planned[item.Target] = true
		}
		items = append(items, item)
	}
	return items, paths, nil
}

func fileExists(fullPath string) bool {
	_, err := os.Stat(fullPath)
	return err == nil
}

// sniffMimeType detects a file's mime type from its first bytes, same as UploadMedia - "" when
// it can't be read or is empty.
func sniffMimeType(fullPath string) string {
	f, err := os.Open(fullPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := io.ReadFull(f, buf)
	if n == 0 {
		return ""
	}
	return http.DetectContentType(buf[:n])
}

// relinkDocs rewrites, in every markdown doc of paths, links resolving to a key of moved
// (docs-relative source) to its media target. Returns how many docs were updated / failed.
func relinkDocs(key logging.Key, paths []string, moved map[string]string) (updated, failed int) {
	idx := newRelocateIndex(paths, moved)

	for _, doc := range paths {
		if !parser.IsMarkdownExtension(doc) {
			continue
		}
		fullPath := pathutils.ToDocsPath("docs/" + doc)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			logging.LogWarning(key, "failed to read %s: %v", doc, err)
			continue
		}

		content, changed := parser.RewriteLinks(string(data), idx.relinkFunc(doc))
		if !changed {
			continue
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			logging.LogError(key, "failed to write %s: %v", doc, err)
			failed++
			continue
		}
		if err := MetaDataSyncNoRefresh("docs/" + doc); err != nil {
			logging.LogWarning(key, "failed to update link metadata for %s: %v", doc, err)
		}
		logging.LogInfo(key, "updated media links in %s", doc)
		updated++
	}
	return updated, failed
}

// relocateIndex resolves link paths written in a doc to a moved source.
type relocateIndex struct {
	moved    map[string]string // docs-relative source -> media target
	existing map[string]bool   // every docs file
}

func newRelocateIndex(paths []string, moved map[string]string) *relocateIndex {
	idx := &relocateIndex{moved: moved, existing: make(map[string]bool, len(paths))}
	for _, p := range paths {
		idx.existing[p] = true
	}
	return idx
}

// relinkFunc returns the parser.RewriteLinks callback pointing doc's links to moved media at their
// /media/ link path (media/ path for wiki links) - decoded, RewriteLinks encodes it.
func (idx *relocateIndex) relinkFunc(doc string) func(l parser.Link) (string, bool) {
	return func(l parser.Link) (string, bool) {
		src := idx.resolve(doc, l)
		if src == "" {
			return "", false
		}
		if l.Kind == parser.LinkWiki {
			return "media/" + idx.moved[src], true
		}
		return "/media/" + idx.moved[src], true
	}
}

// resolve resolves a link written in doc to a key of moved: a "./" or "../" link strictly relative
// to doc's folder (pathutils.ResolveRelativeLink, like they render), a wiki or bare markdown link relative
// to the docs root, like they render (a bare markdown one falls back to doc's folder, the way
// imported wikis write it), a relative html link strictly relative to doc's folder, a root ("/",
// wiki.js) link relative to doc's folder first, then to each parent folder up to the docs root.
// For all but html relative links "docs/", "/files/" or "/files/docs/" means docs root,
// unless a real folder of that name matches. A "/media/" link to an existing media file is left
// alone. Stops at the first candidate that is an existing, not moved file, so a link never
// gets redirected to a same-named file higher up.
func (idx *relocateIndex) resolve(doc string, l parser.Link) string {
	link := l.Path
	root := strings.HasPrefix(link, "/")
	if root && strings.HasPrefix(link, "/media/") {
		if _, err := os.Stat(pathutils.ToMediaPath(link)); err == nil {
			return ""
		}
	}

	// root links ("/upload/x.png") from a wiki imported into a subfolder are relative to that
	// subfolder, so they try doc's folder and each parent up to the docs root
	var dirs []string
	switch resolved := pathutils.ResolveRelativeLink("docs/"+doc, link); {
	case resolved != link:
		link, dirs = resolved, []string{"."}
	case root:
		for dir := path.Dir(doc); ; dir = path.Dir(dir) {
			if dirs = append(dirs, dir); dir == "." {
				break
			}
		}
	case l.Kind == parser.LinkWiki:
		dirs = []string{"."}
	case l.Kind == parser.LinkMarkdown:
		dirs = []string{".", path.Dir(doc)}
	default:
		dirs = []string{path.Dir(doc)}
	}
	for _, dir := range dirs {
		c := strings.TrimPrefix(path.Clean("/"+path.Join(dir, link)), "/")
		candidates := []string{c}
		if root || l.Kind != parser.LinkHTML {
			for _, prefix := range []string{"files/docs/", "files/", "docs/"} {
				if s, ok := strings.CutPrefix(c, prefix); ok {
					candidates = append(candidates, s)
				}
			}
		}
		for _, p := range candidates {
			if _, ok := idx.moved[p]; ok {
				return p
			}
			if idx.existing[p] {
				return ""
			}
		}
	}
	return ""
}
