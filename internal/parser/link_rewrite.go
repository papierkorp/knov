package parser

import (
	"cmp"
	"html"
	"net/url"
	"os"
	"regexp"
	"slices"
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
// starts with "<" (CommonMark, [x](<a.md) or [x]( <a.md) is no link)
const mdLinkDestPattern = `([ \t]*(?:<[^>\n]*>[^)\n]*|(?:[^()\s<]|\([^()\n]*\))(?:[^()\n]|\([^()\n]*\))*))`

var (
	// a [[wikilink]] (group 1) or [text](dest) / ![alt](dest) (the text may hold escaped
	// brackets, group 2, the destination group 3) in one pass, so a "](" closing a wikilink is no
	// link ("[[a]](b)" is [[a]] and the text "(b)"). The text may span lines and hold code but no
	// unescaped "[", so a nested ![img](src) or one after a stray "[" matches on its own; the
	// outer link of nested brackets matches as "](dest)" alone.
	mdLinkRe          = regexp.MustCompile(wikiLinkPattern + `|(!?\[(?:[^\[\]\\]|\\.)*)?\]\(` + mdLinkDestPattern + `\)`)
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
// destination, kept so its query / anchor may hold spaces. Text is only used by String, for
// writing a new link, Image by String and set by the link walker (walkLinks) for a markdown
// image. ParseLink reads one, Dest and String write it - the only place link paths are decoded
// and encoded.
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

// RewriteLinks replaces the path of every link walkLinks finds (not external ones) for which fn
// returns a new path: fn gets the link as read by ParseLink and returns the new decoded path, the
// link is written back with Dest only if the path changed - the rest of the content is kept
// as-is. Returns the content and whether anything was replaced.
// ExtractLinks walks links through this too, so both always see the same links.
func RewriteLinks(content string, fn func(l Link) (string, bool)) (string, bool) {
	changed := false
	content = walkLinks(content, func(m linkMatch) string {
		l := m.Link
		if l.External {
			return m.whole()
		}
		newPath, ok := fn(l)
		if !ok || newPath == l.Path {
			return m.whole()
		}
		changed = true
		l.Path = newPath
		return m.Prefix + l.Dest() + m.Suffix
	})
	return content, changed
}

// linkMatch is one link walkLinks found: Link as read by ParseLink (Image set for a markdown
// image), Open the "[text" / "![alt" of a markdown ](dest) ("" without one: the outer link of
// nested brackets), RefDef for a reference definition. The matched text is Prefix + Dest +
// Suffix: "[[" body "]]", "[text](" dest ")", "[id]: " dest, `<img src="` value.
type linkMatch struct {
	Link                       Link
	Open, Prefix, Dest, Suffix string
	RefDef                     bool
}

func (m linkMatch) whole() string { return m.Prefix + m.Dest + m.Suffix }

// walkLinks replaces every markdown [text](dest) / ](dest), [[wikilink]], reference-style
// [id]: dest and html src/href outside fenced code blocks and inline code with fn(m) - the one
// link scanner, RewriteLinks, ExtractLinks and RenderLinks only differ in what they write back.
// The content is masked once (maskCode): markdown and wiki links are matched on the whole
// content (a link text may span lines and hold code, a destination holding code is no link),
// reference definitions (only on a line without code) and html attributes line by line. A link
// overlapping an earlier one (by start, a reference definition before a markdown link before an
// html attribute) is no link.
func walkLinks(content string, fn func(m linkMatch) string) string {
	type span struct {
		start, end, order int
		m                 linkMatch
	}
	var spans []span
	masked := maskCode(content)
	for _, i := range mdLinkRe.FindAllStringSubmatchIndex(masked, -1) {
		if i[2] != -1 {
			body := content[i[2]:i[3]]
			spans = append(spans, span{i[0], i[1], 1, linkMatch{Link: ParseLink(body, LinkWiki), Prefix: "[[", Dest: body, Suffix: "]]"}})
			continue
		}
		if strings.Contains(masked[i[6]:i[7]], "\x00") {
			continue
		}
		m := linkMatch{Prefix: content[i[0]:i[6]], Dest: content[i[6]:i[7]], Suffix: ")"}
		if i[4] != -1 {
			m.Open = content[i[4]:i[5]]
		}
		m.Link = ParseLink(m.Dest, LinkMarkdown)
		m.Link.Image = strings.HasPrefix(m.Open, "!")
		spans = append(spans, span{i[0], i[1], 1, m})
	}
	offset := 0
	for _, line := range strings.Split(masked, "\n") {
		text := content[offset : offset+len(line)]
		if i := rewriteRefDefRe.FindStringSubmatchIndex(text); i != nil && !strings.Contains(line, "\x00") {
			m := linkMatch{Prefix: text[i[2]:i[3]], Dest: text[i[4]:i[5]], RefDef: true}
			m.Link = ParseLink(m.Dest, LinkMarkdown)
			spans = append(spans, span{offset, offset + len(text), 0, m})
		}
		for _, i := range rewriteHTMLAttrRe.FindAllStringSubmatchIndex(line, -1) {
			if !strings.Contains(line[i[0]:i[1]], "\x00") {
				m := linkMatch{Prefix: text[i[2]:i[3]], Dest: text[i[4]:i[5]]}
				m.Link = ParseLink(m.Dest, LinkHTML)
				spans = append(spans, span{offset + i[0], offset + i[1], 2, m})
			}
		}
		offset += len(line) + 1
	}
	slices.SortStableFunc(spans, func(a, b span) int {
		return cmp.Or(a.start-b.start, a.order-b.order)
	})

	var b strings.Builder
	last := 0
	for _, s := range spans {
		if s.start < last {
			continue
		}
		b.WriteString(content[last:s.start])
		b.WriteString(fn(s.m))
		last = s.end
	}
	b.WriteString(content[last:])
	return b.String()
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
// The path is read like ResolveLinkPath: a bare markdown / html one and a "./" or "../" one from
// docPath's folder, a wikilink and a leading "/" (a /files/ url too) from the docs root, a
// "media/" or "/media/" one from the media folder; a path without "media/" naming an existing
// media file is that media file (links copied from the media page have no "media/" prefix).
func LinkTarget(docPath string, l Link) string {
	return linkTarget(ResolveLinkPath(docPath, l), l)
}

// DocsRootLinkTarget is the LinkTarget l had while a bare markdown or html path was read from the
// docs root - for the relative links migration (files.ScanRelativeLinks).
func DocsRootLinkTarget(docPath string, l Link) string {
	return linkTarget(pathutils.ResolveRelativeLink(docPath, l.Path), l)
}

// linkTarget is the LinkTarget of l with its path p read from the docs root.
func linkTarget(p string, l Link) string {
	if l.External || l.Path == "" || IsAppRouteLink(l.Path, l.Kind) {
		return ""
	}
	p = utils.NormalizeLinkPath(p)
	if !strings.HasPrefix(p, "media/") && !strings.HasPrefix(p, "docs/") {
		if _, err := os.Stat(pathutils.ToMediaPath(p)); err == nil {
			p = "media/" + p
		}
	}
	return pathutils.ToWithPrefix(p)
}

// IsBareLink reports whether l is a markdown or html link whose path has no "./", "../", leading
// "/" or "media/" - read from the folder of its doc like "./" (CommonMark), see LinkTarget.
func IsBareLink(l Link) bool {
	return l.Kind != LinkWiki && !l.External && l.Path != "" && !strings.HasPrefix(l.Path, "/") &&
		!strings.HasPrefix(l.Path, "media/") && !pathutils.IsRelativeLink(l.Path)
}

// ResolveLinkPath is the path of l written in the doc docPath, read from the docs root: a bare
// (IsBareLink), "./" or "../" one resolved against docPath's folder (pathutils.ResolveRelativeLink,
// "/" for the docs root itself), any other as written.
func ResolveLinkPath(docPath string, l Link) string {
	if IsBareLink(l) {
		return pathutils.ResolveRelativeLink(docPath, "./"+l.Path)
	}
	return pathutils.ResolveRelativeLink(docPath, l.Path)
}
