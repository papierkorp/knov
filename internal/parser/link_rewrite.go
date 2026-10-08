package parser

import (
	"html"
	"net/url"
	"os"
	"regexp"
	"strings"

	"knov/internal/markdown"
	"knov/internal/pathutils"
	"knov/internal/pathutils/crosspath"
	"knov/internal/utils"
)

// LinkKind is the syntax a link was written in.
type LinkKind int

const (
	LinkMarkdown LinkKind = iota // ](dest) and reference-style [id]: dest
	LinkWiki                     // [[path|text]]
	LinkHTML                     // <img/a/video/audio/source src/href="...">
)

// a [[wikilink]] body never holds brackets, line breaks or masked code (see maskCode)
const wikiLinkPattern = `\[\[([^\[\]\n\x00]+)\]\]`

// a markdown link destination: <...> plus title, or one level of (...) in it - a bare one never
// starts with "<" (CommonMark, [x](<a.md) or [x]( <a.md) is no link). shared by RewriteLinks and
// RenderLinks so both see the same links
const mdLinkDestPattern = `([ \t]*(?:<[^>\n]*>[^)\n]*|(?:[^()\s<]|\([^()\n]*\))(?:[^()\n]|\([^()\n]*\))*))`

var (
	// a [[wikilink]] (group 1) or a markdown ](dest) (group 2) in one pass, so a "](" closing a
	// wikilink is never read as a markdown link - "[[a]](b)" is [[a]] followed by the text "(b)"
	rewriteLinkRe     = regexp.MustCompile(wikiLinkPattern + `|\]\(` + mdLinkDestPattern + `\)`)
	rewriteHTMLAttrRe = regexp.MustCompile(`(<(?i:img|a|video|audio|source)(?:\s[^>]*?)?\s(?i:src|href)\s*=\s*["'])([^"'\n]+)`)
	// [id]: dest, not [^footnote]: - dest is <...> or has no spaces, only a size / title may follow, so prose like "[note]: remember this" isn't a link
	rewriteRefDefRe = regexp.MustCompile(`^( {0,3}\[[^\]^][^\]]*\]:[ \t]*)((?:<[^>\n]*>|[^<\s]\S*)(?:[ \t]+=\d*x\d*)?(?:[ \t]+(?:"[^"\n]*"|'[^'\n]*'|\([^)\n]*\)))?[ \t\r]*)$`)
	// a line starting a new block (heading, list item, quote, table row) - a code span doesn't continue onto it
	blockStartRe      = regexp.MustCompile(`^ {0,3}(?:#{1,6}(?:[ \t]|$)|[-*+][ \t]|\d{1,9}[.)][ \t]|>|\|)`)
	linkSuffixRe      = regexp.MustCompile(`\s+(?:=\d*x\d*|["'])`) // " =WxH" (wiki.js) or "title"
	wikijsImageSizeRe = regexp.MustCompile(`^\s+=\d*x\d*`)
	// a CommonMark backslash escape (\_ \( ...) - except "\.", so windows "..\" and ".hidden"
	// segments (sub\..\x, sub\.git) count as separators
	mdEscapeRe = regexp.MustCompile(`\\[!-\-/:-@\[-` + "`" + `{-~]`)
)

// isWindowsPath reports whether a markdown link path uses windows "\" separators - any "\" that
// isn't a markdown escape (sub\note, a\..\b) makes all of its "\" separators (sub\_resources\x).
func isWindowsPath(p string) bool {
	return strings.Contains(mdEscapeRe.ReplaceAllString(p, ""), `\`)
}

// markdownLinkPath resolves the "\" of a markdown link path: separators in a windows path,
// CommonMark escapes otherwise (\_ -> _). An escaped "\\" is a separator too, a "\" is never
// part of a linked filename.
func markdownLinkPath(p string) string {
	if !isWindowsPath(p) {
		p = mdEscapeRe.ReplaceAllStringFunc(p, func(m string) string { return m[1:] })
	}
	return crosspath.ToSlash(p)
}

// Link is one link as written in content. Path is the decoded file path ("" for a pure "#anchor"
// link, as written for an External one), everything else is kept as written: Query "?...",
// Anchor "#...", Alias "|text" of a wikilink, Title the " title" after a markdown destination (a
// wiki.js " =WxH" image size is dropped, goldmark doesn't parse it), Angle a markdown <...>
// destination, kept so its query / anchor may hold spaces. Text and Image are only used by
// String, for writing a new link. ParseLink reads one, Dest and String write it - the only place
// link paths are decoded and encoded.
type Link struct {
	Kind     LinkKind
	Image    bool
	Text     string
	Path     string
	Query    string
	Anchor   string
	Alias    string
	Title    string
	Angle    bool
	External bool
}

// ParseLink reads a link destination as written: the dest of a markdown ](dest) or [id]: dest,
// the body of a [[wikilink]] or an html src/href value. The path ends at "?" (not in a
// wikilink), "#", a wikilink's "|", the ">" of a <...> destination (a title may follow it) or a
// markdown title; it is
// trimmed as written, then decoded (decodeLinkPath).
func ParseLink(dest string, kind LinkKind) Link {
	l := Link{Kind: kind}
	if kind == LinkWiki {
		var rest string
		if i := strings.IndexAny(dest, "#|"); i != -1 {
			dest, rest = dest[:i], dest[i:]
		}
		if i := strings.Index(rest, "|"); i != -1 {
			rest, l.Alias = rest[:i], rest[i:]
		}
		l.Anchor = rest
		return l.withPath(strings.Trim(dest, " ")) // only spaces, like encodeLinkPath protects them
	}

	dest = strings.TrimLeft(dest, " \t")
	// title (and size) and trailing whitespace: after the ">" of a <...> destination, else from
	// the first " title" / " =WxH" - what is left is "path?query#anchor"
	// the angle branch only with a closing ">" - an unclosed "<" is part of the path
	if inner, title, ok := strings.Cut(dest, ">"); ok && kind == LinkMarkdown && strings.HasPrefix(dest, "<") {
		dest, l.Title, l.Angle = inner[1:], wikijsImageSizeRe.ReplaceAllString(title, ""), true
	} else {
		body := strings.TrimRight(dest, " \t\r") // \r: crlf line ending
		l.Title = dest[len(body):]
		if loc := linkSuffixRe.FindStringIndex(body); kind == LinkMarkdown && loc != nil {
			body, l.Title = body[:loc[0]], wikijsImageSizeRe.ReplaceAllString(body[loc[0]:], "")+l.Title
		}
		dest = body
	}
	end := strings.IndexAny(dest, "?#")
	if end == -1 {
		end = len(dest)
	}
	p := strings.TrimRight(dest[:end], " \t\r")
	rest := dest[len(p):]
	if i := strings.Index(rest, "#"); i != -1 {
		rest, l.Anchor = rest[:i], rest[i:]
	}
	l.Query = rest
	return l.withPath(p)
}

// withPath sets the path as written: decoded, or kept for an external link.
func (l Link) withPath(p string) Link {
	if l.External = isExternalLink(p, l.Kind); l.External {
		l.Path = p
	} else {
		l.Path = decodeLinkPath(p, l.Kind)
	}
	return l
}

// Dest writes the link destination (wikilink body) so ParseLink reads it back unchanged.
func (l Link) Dest() string {
	p := l.Path
	if !l.External {
		p = encodeLinkPath(p, l.Kind)
	}
	if l.Kind == LinkWiki {
		return p + l.Anchor + l.Alias
	}
	if l.Angle {
		return "<" + p + l.Query + l.Anchor + ">" + l.Title
	}
	return p + l.Query + l.Anchor + l.Title
}

// AnchorText is the anchor without its "#", percent-decoded - the heading text or id it points
// at ("" without anchor). The only place an anchor is decoded, see AnchorID.
func (l Link) AnchorText() string {
	a := strings.TrimPrefix(strings.TrimSpace(l.Anchor), "#")
	if decoded, err := url.PathUnescape(a); err == nil {
		return decoded
	}
	return a
}

// String writes a new markdown [text](dest) / ![alt](dest) or a [[wikilink]].
func (l Link) String() string {
	if l.Kind == LinkWiki {
		return "[[" + l.Dest() + "]]"
	}
	prefix := ""
	if l.Image {
		prefix = "!"
	}
	return prefix + "[" + linkTextEscaper.Replace(l.Text) + "](" + l.Dest() + ")"
}

// FileLinkDest writes the destination of a new link to a docs file (docs-relative path, anchor
// "#id" or ""), as the editor inserts it: the wikilink body or a markdown /files/ url - an empty
// path is a same-page anchor.
func FileLinkDest(path, anchor string, kind LinkKind) string {
	if kind != LinkWiki && path != "" {
		path = "/files/" + path
	}
	return Link{Kind: kind, Path: path, Anchor: anchor}.Dest()
}

// decodeLinkPath turns a link path as written into the file path it points at, the way the
// renderer reads it: windows "\" separators as "/", for markdown CommonMark escapes resolved,
// for markdown and html entities resolved (goldmark and the browser do), then percent-decoded once.
func decodeLinkPath(p string, kind LinkKind) string {
	switch kind {
	case LinkMarkdown:
		p = entityRe.ReplaceAllStringFunc(markdownLinkPath(p), html.UnescapeString)
	case LinkHTML:
		p = entityRe.ReplaceAllStringFunc(p, html.UnescapeString)
	}
	return unescapePath(p)
}

// a complete html character reference - like goldmark, not the legacy ones without ";"
var entityRe = regexp.MustCompile(`&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[a-zA-Z][a-zA-Z0-9]{1,31});`)

// unescapePath percent-decodes a link path once, with windows "\" separators as "/".
func unescapePath(p string) string {
	p = crosspath.ToSlash(p)
	if decoded, err := url.PathUnescape(p); err == nil {
		return decoded
	}
	return p
}

// percent-encode only what decodeLinkPath would change or what would end the path early: "%",
// "\", "&" (entity), anchor, line breaks, for markdown the query and what ends a bare or <...>
// destination (with spaces encoded a quote can't start a title), for html also quotes, for a
// wikilink its "|" / "[" / "]" and leading / trailing spaces (the rest of its spaces stays readable)
var (
	mdLinkPathEscaper   = strings.NewReplacer("%", "%25", `\`, "%5C", "&", "%26", "#", "%23", "?", "%3F", "\r", "%0D", "\n", "%0A", " ", "%20", "\t", "%09", "(", "%28", ")", "%29", "<", "%3C", ">", "%3E")
	htmlLinkPathEscaper = strings.NewReplacer("%", "%25", `\`, "%5C", "&", "%26", "#", "%23", "?", "%3F", "\r", "%0D", "\n", "%0A", " ", "%20", "\t", "%09", `"`, "%22", "'", "%27", "<", "%3C", ">", "%3E")
	wikiLinkPathEscaper = strings.NewReplacer("%", "%25", `\`, "%5C", "#", "%23", "\r", "%0D", "\n", "%0A", "|", "%7C", "[", "%5B", "]", "%5D")
	linkTextEscaper     = strings.NewReplacer(`\`, `\\`, "[", `\[`, "]", `\]`)
)

// encodeLinkPath writes a file path as a link path that decodeLinkPath reads back unchanged.
func encodeLinkPath(p string, kind LinkKind) string {
	switch kind {
	case LinkWiki:
		p = wikiLinkPathEscaper.Replace(p)
		trimmed := strings.TrimLeft(p, " ")
		p = strings.Repeat("%20", len(p)-len(trimmed)) + trimmed
		trimmed = strings.TrimRight(p, " ")
		return trimmed + strings.Repeat("%20", len(p)-len(trimmed))
	case LinkHTML:
		p = htmlLinkPathEscaper.Replace(p)
	default:
		p = mdLinkPathEscaper.Replace(p)
	}
	// a ":" only reads as a scheme before the first "/"
	seg, _, _ := strings.Cut(p, "/")
	return strings.Replace(p, ":", "%3A", strings.Count(seg, ":"))
}

// RewriteLinks replaces, outside fenced code blocks and inline code, the path of every markdown
// ](dest), [[wiki]], reference-style [id]: dest and html src/href link (not external ones) for
// which fn returns a new path: fn gets the link as read by ParseLink and returns the new decoded
// path, the link is written back with Dest only if the path changed - the rest of the content
// is kept as-is. Returns the content and whether anything was replaced.
// ExtractLinks walks links through this too, so both always see the same links.
func RewriteLinks(content string, fn func(l Link) (string, bool)) (string, bool) {
	changed := false
	replace := func(dest string, kind LinkKind) string {
		l := ParseLink(dest, kind)
		if l.External {
			return dest
		}
		newPath, ok := fn(l)
		if !ok || newPath == l.Path {
			return dest
		}
		changed = true
		l.Path = newPath
		return l.Dest()
	}

	content = replaceOutsideCode(content, func(part string, wholeLine bool) string {
		if sub := rewriteRefDefRe.FindStringSubmatch(part); wholeLine && sub != nil {
			return sub[1] + replace(sub[2], LinkMarkdown)
		}
		var b strings.Builder
		last := 0
		for _, m := range rewriteLinkRe.FindAllStringSubmatchIndex(part, -1) {
			b.WriteString(part[last:m[0]])
			if m[2] != -1 {
				b.WriteString("[[" + replace(part[m[2]:m[3]], LinkWiki) + "]]")
			} else {
				b.WriteString("](" + replace(part[m[4]:m[5]], LinkMarkdown) + ")")
			}
			last = m[1]
		}
		part = b.String() + part[last:]
		return rewriteHTMLAttrRe.ReplaceAllStringFunc(part, func(m string) string {
			// the path can't contain quotes, so the last one ends the tag prefix
			i := strings.LastIndexAny(m, `"'`) + 1
			return m[:i] + replace(m[i:], LinkHTML)
		})
	})
	return content, changed
}

// replaceOutsideCode replaces every part of a line outside fenced code blocks and inline `code`
// spans (maskCode) with fn(part, wholeLine) - the link scanner of RewriteLinks and the renderer, so
// code is never a link. wholeLine is false for a line with a code span, so it's never a reference definition.
func replaceOutsideCode(content string, fn func(part string, wholeLine bool) string) string {
	lines := strings.Split(content, "\n")
	masked := strings.Split(maskCode(content), "\n")
	for i, line := range lines {
		var b strings.Builder
		for start := 0; start < len(line); {
			code, end := masked[i][start] == 0, start
			for end < len(line) && (masked[i][end] == 0) == code {
				end++
			}
			if code {
				b.WriteString(line[start:end])
			} else {
				b.WriteString(fn(line[start:end], end-start == len(line)))
			}
			start = end
		}
		lines[i] = b.String()
	}
	return strings.Join(lines, "\n")
}

// maskCode replaces every byte of fenced code blocks and inline `code` spans with "\x00" (line
// breaks kept), so a regex over the result never sees code and its indexes match content. a code
// span may span the lines of a paragraph, but not a blank line, a fence, a heading or a new list
// item, quote or table row (blockStartRe).
func maskCode(content string) string {
	lines := strings.Split(content, "\n")
	fenced := markdown.FenceMask(lines)
	for i := 0; i < len(lines); {
		j := i
		for j < len(lines) && !fenced[j] && strings.TrimSpace(lines[j]) != "" && (j == i || !blockStartRe.MatchString(lines[j]) && !isATXHeading(lines[j-1])) {
			j++
		}
		if j == i {
			if fenced[i] {
				lines[i] = strings.Repeat("\x00", len(lines[i]))
			}
			i++
			continue
		}
		parts := markdown.SplitCodeSpans(strings.Join(lines[i:j], "\n"))
		for k := 1; k < len(parts); k += 2 {
			b := []byte(parts[k])
			for x := range b {
				if b[x] != '\n' {
					b[x] = 0
				}
			}
			parts[k] = string(b)
		}
		copy(lines[i:j], strings.Split(strings.Join(parts, ""), "\n"))
		i = j
	}
	return strings.Join(lines, "\n")
}

// isATXHeading reports whether line is a "# heading" - its own block, a code span doesn't continue after it.
func isATXHeading(line string) bool {
	_, _, ok := markdown.ATXHeading(line)
	return ok
}

// IsAppRouteLink reports whether a link is an html root link to an app route (/dashboard,
// /search?q=...) rather than a file - for html only /media/ and /files/ are files.
func IsAppRouteLink(p string, kind LinkKind) bool {
	return kind == LinkHTML && strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "/media/") && !strings.HasPrefix(p, "/files/")
}

// LinkTarget is the file the link l written in the doc docPath points at, as metadata path
// ("docs/a.md", "media/x.png", "docs/sub/" for a folder) - "" for an external link, a pure
// anchor or an html link to an app route. The one place a link path is resolved: rendering,
// link metadata, rename, move and media relocation all read the target from here.
// A "./" or "../" path is read from docPath's folder, any other from the docs root (a /files/
// url too), a "media/" or "/media/" one from the media folder; a bare path naming an existing
// media file is that media file (links copied from the media page have no "media/" prefix).
func LinkTarget(docPath string, l Link) string {
	if l.External || l.Path == "" || IsAppRouteLink(l.Path, l.Kind) {
		return ""
	}
	p := utils.NormalizeLinkPath(pathutils.ResolveRelativeLink(docPath, l.Path))
	if !strings.HasPrefix(p, "media/") && !strings.HasPrefix(p, "docs/") {
		if _, err := os.Stat(pathutils.ToMediaPath(p)); err == nil {
			p = "media/" + p
		}
	}
	return pathutils.ToWithPrefix(p)
}
