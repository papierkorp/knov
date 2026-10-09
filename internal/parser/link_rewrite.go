package parser

import (
	"cmp"
	"fmt"
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
// starts with "<" (CommonMark, [x](<a.md) or [x]( <a.md) is no link). One line break is allowed
// before the destination and before a title.
const mdLinkDestPattern = `([ \t]*(?:\n[ \t]*)?(?:<[^>\n]*>[^)\n]*|(?:[^()\s<]|\([^()\n]*\))(?:[^()\n]|\([^()\n]*\))*)(?:[ \t]*\n[ \t]*(?:"[^"\n]*"|'[^'\n]*'))?)`

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
	// a reference definition whose destination is on the next line: "[id]:" alone, then the destination part
	rewriteRefDefOpenRe = regexp.MustCompile(`^ {0,3}\[[^\]^][^\]]*\]:[ \t]*$`)
	rewriteRefDefDestRe = regexp.MustCompile(`^([ \t]*)((?:<[^>\n]*>|[^<\s]\S*)(?:[ \t]+=\d*x\d*)?(?:[ \t]+(?:"[^"\n]*"|'[^'\n]*'|\([^)\n]*\)))?[ \t\r]*)$`)
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

	dest = strings.TrimLeft(dest, " \t\r\n")
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

// DocsWikiPath is the wikilink path of the docs file rel (docs-relative): a wikilink reads media/
// and docs/ as the media folder and the docs prefix, so a file in a folder of that name needs its
// docs/ prefix.
func DocsWikiPath(rel string) string {
	if strings.HasPrefix(rel, "media/") || strings.HasPrefix(rel, "docs/") {
		return "docs/" + rel
	}
	return rel
}

// FileLinkDest writes the destination of a new link to a docs file (docs-relative path, anchor
// "#id" or ""), as the editor inserts it: the wikilink body or a markdown /files/ url - an empty
// path is a same-page anchor.
func FileLinkDest(path, anchor string, kind LinkKind) string {
	if kind != LinkWiki && path != "" {
		path = "/files/" + path
	} else {
		path = DocsWikiPath(path)
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
// "\", "&" (entity), anchor, ascii control characters (a "\f" ends a markdown destination), "`"
// (two of them are a code span for the link walker), line breaks, for markdown the query and what
// ends a bare or <...> destination (with spaces encoded a quote can't start a title), for html
// also quotes, for a wikilink its "|" / "[" / "]" and leading / trailing spaces (the rest of its
// spaces stays readable)
var (
	mdLinkPathEscaper   = newLinkPathEscaper("&", "#", "?", " ", "(", ")", "<", ">")
	htmlLinkPathEscaper = newLinkPathEscaper("&", "#", "?", " ", `"`, "'", "<", ">")
	wikiLinkPathEscaper = newLinkPathEscaper("#", "|", "[", "]")
	linkTextEscaper     = strings.NewReplacer(`\`, `\\`, "[", `\[`, "]", `\]`)
)

// newLinkPathEscaper encodes "%", "\", "`", every ascii control character and the extra characters
func newLinkPathEscaper(extra ...string) *strings.Replacer {
	pairs := []string{"%", "%25", `\`, "%5C", "`", "%60"}
	for r := rune(0); r < 0x20; r++ {
		pairs = append(pairs, string(r), fmt.Sprintf("%%%02X", r))
	}
	pairs = append(pairs, "\x7f", "%7F")
	for _, c := range extra {
		pairs = append(pairs, c, fmt.Sprintf("%%%02X", c[0]))
	}
	return strings.NewReplacer(pairs...)
}

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
	var rows tableRows
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
		dest := l.Dest()
		if l.Kind != LinkWiki && strings.Contains(dest, "|") {
			if rows == nil {
				rows = newTableRows(content)
			}
			if rows.contains(m.Start) {
				// GFM ends a table cell at an unescaped "|"; in html the cell unescaping isn't applied to the attribute
				escaped := `\|`
				if l.Kind == LinkHTML {
					escaped = "%7C"
				}
				dest = strings.ReplaceAll(dest, "|", escaped)
			}
		}
		return m.Prefix + dest + m.Suffix
	})
	return content, changed
}

// SoleImageLink returns the local markdown image link text consists of and nothing else (a
// header / footer zone template that embeds an image), ok is false for any other text.
func SoleImageLink(text string) (Link, bool) {
	var found []linkMatch
	walkLinks(text, func(m linkMatch) string {
		found = append(found, m)
		return m.whole()
	})
	if len(found) != 1 || !found[0].Link.Image || found[0].Link.External || found[0].whole() != text {
		return Link{}, false
	}
	return found[0].Link, true
}

// linkMatch is one link walkLinks found: Link as read by ParseLink (Image set for a markdown
// image), Open the "[text" / "![alt" of a markdown ](dest) ("" without one: the outer link of
// nested brackets), RefDef for a reference definition. The matched text is Prefix + Dest +
// Suffix: "[[" body "]]", "[text](" dest ")", "[id]: " dest, `<img src="` value.
type linkMatch struct {
	Link                       Link
	Open, Prefix, Dest, Suffix string
	RefDef                     bool
	Start                      int // offset of the match in the content
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
		if strings.Contains(masked[i[6]:i[7]], "\x00") || i[4] != -1 && escapedAt(masked, i[4]+strings.IndexByte(masked[i[4]:i[5]], '[')) {
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
	maskedLines := strings.Split(masked, "\n")
	for n, line := range maskedLines {
		text := content[offset : offset+len(line)]
		if i := rewriteRefDefRe.FindStringSubmatchIndex(text); i != nil && !strings.Contains(line, "\x00") {
			m := linkMatch{Prefix: text[i[2]:i[3]], Dest: text[i[4]:i[5]], RefDef: true}
			m.Link = ParseLink(m.Dest, LinkMarkdown)
			spans = append(spans, span{offset, offset + len(text), 0, m})
		} else if rewriteRefDefOpenRe.MatchString(text) && !strings.Contains(line, "\x00") && n+1 < len(maskedLines) && !strings.Contains(maskedLines[n+1], "\x00") {
			// the destination on the next line
			next := content[offset+len(line)+1 : offset+len(line)+1+len(maskedLines[n+1])]
			if j := rewriteRefDefDestRe.FindStringSubmatchIndex(next); j != nil {
				m := linkMatch{Prefix: text + "\n" + next[j[2]:j[3]], Dest: next[j[4]:j[5]], RefDef: true}
				m.Link = ParseLink(m.Dest, LinkMarkdown)
				spans = append(spans, span{offset, offset + len(text) + 1 + len(next), 0, m})
			}
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
		s.m.Start = s.start
		b.WriteString(fn(s.m))
		last = s.end
	}
	b.WriteString(content[last:])
	return b.String()
}

// escapedAt reports whether the byte at i is escaped by an odd number of backslashes before it.
func escapedAt(s string, i int) bool {
	n := 0
	for i--; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// maskCode replaces every byte of fenced code blocks and inline `code` spans with "\x00" (line
// breaks kept), so a regex over the result never sees code and its indexes match content. a code
// span may span the lines of a paragraph, but not a blank line, a fence, a heading or a new list
// item, quote or table row (blockStartRe).
func maskCode(content string) string {
	lines := strings.Split(content, "\n")
	fenced := markdown.FenceMask(lines)
	for i, indented := range indentedCodeMask(lines, fenced) {
		fenced[i] = fenced[i] || indented
	}
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
		parts := markdown.SplitCodeSpans(protectDestBackticks(strings.Join(lines[i:j], "\n")))
		for k := 0; k < len(parts); k++ {
			if k%2 == 0 {
				parts[k] = strings.ReplaceAll(parts[k], "\x01", "`")
				continue
			}
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
	return htmlCommentRe.ReplaceAllStringFunc(strings.Join(lines, "\n"), maskNonNewlines)
}

// an html comment (inline, or a block starting a line and running to the end of the content if it
// is never closed), where markdown links aren't read
var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->|(?m)^ {0,3}<!--.*\z`)

func maskNonNewlines(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] != '\n' {
			b[i] = 0
		}
	}
	return string(b)
}

var destBacktickRe = regexp.MustCompile(`\]\(` + mdLinkDestPattern + `\)`)

// protectDestBackticks hides the backticks in a link destination from the code span scan: goldmark
// reads a destination before any code span ("[x](a`b`.md)" is a link to a`b`.md).
func protectDestBackticks(s string) string {
	return destBacktickRe.ReplaceAllStringFunc(s, func(m string) string {
		return strings.ReplaceAll(m, "`", "\x01")
	})
}

var (
	listMarkerRe = regexp.MustCompile(`^( *)([-*+]|\d{1,9}[.)])( {1,4}|$)`)
	quotePrefix  = regexp.MustCompile(`^ {0,3}>[ ]?`)
)

// indentedCodeMask marks the lines of indented code blocks (4 spaces or a tab more than the content
// of their list item, after a blank line or another code line - never continuing a paragraph),
// where markdown links aren't read. Fenced lines are skipped.
func indentedCodeMask(lines []string, fenced []bool) []bool {
	mask := make([]bool, len(lines))
	prevBlank, prevPara, inCode := true, false, false
	listContent := -1
	for i, line := range lines {
		if fenced[i] {
			prevBlank, prevPara, inCode = false, false, false
			continue
		}
		for quotePrefix.MatchString(line) {
			line = line[len(quotePrefix.FindString(line)):]
		}
		if strings.TrimSpace(line) == "" {
			prevBlank = true
			continue
		}
		indent := 0
		for _, c := range line {
			if c == ' ' {
				indent++
			} else if c == '\t' {
				indent += 4 - indent%4
			} else {
				break
			}
		}
		if m := listMarkerRe.FindStringSubmatch(line); m != nil && indent-max(listContent, 0) < 4 {
			listContent = len(m[1]) + len(m[2]) + len(m[3])
			if len(m[3]) > 4 || m[3] == "" {
				listContent = len(m[1]) + len(m[2]) + 1
			}
			prevBlank, prevPara, inCode = false, strings.TrimSpace(line[len(m[0]):]) != "", false
			continue
		}
		if listContent >= 0 && indent < listContent {
			if prevBlank {
				listContent = -1
			} else {
				prevPara, inCode = true, false
				continue
			}
		}
		base := max(listContent, 0)
		if indent-base >= 4 && (prevBlank || inCode || !prevPara) {
			mask[i], inCode, prevBlank, prevPara = true, true, false, false
			continue
		}
		inCode, prevBlank = false, false
		prevPara = !isATXHeading(line)
	}
	return mask
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
// docPath's folder (a docs file even when that folder is named docs, media or files), a wikilink
// and a leading "/" from the docs root, a "/files/" url as the docs-relative path taken literally
// ("/files/media/x.md" is the docs file docs/media/x.md), a path written "media/", "./media/" or
// "/media/" from the media folder and one written "docs/" as that docs path ("[[docs/media/x]]");
// a path without a prefix naming an existing media file is that media file (links copied from the
// media page have no "media/" prefix).
func LinkTarget(docPath string, l Link) string {
	return linkTarget(ResolveLinkPath(docPath, l), l, IsBareLink(l) || pathutils.IsRelativeLink(l.Path))
}

// DocsRootLinkTarget is the LinkTarget l had while a bare markdown or html path was read from the
// docs root - for the relative links migration (files.ScanRelativeLinks).
func DocsRootLinkTarget(docPath string, l Link) string {
	return linkTarget(pathutils.ResolveRelativeLink(docPath, l.Path), l, pathutils.IsRelativeLink(l.Path))
}

// linkTarget is the LinkTarget of l with its path p read from the docs root; joined is whether p
// was resolved from the doc's folder, which makes it a docs-root path whatever its first folder
// is called - unless l is written as a media link.
func linkTarget(p string, l Link, joined bool) string {
	if l.External || l.Path == "" || IsAppRouteLink(l.Path, l.Kind) {
		return ""
	}
	if p == "/" {
		return "docs/" // the docs root itself
	}
	p = utils.NormalizeLinkPath(p)
	switch {
	case joined && !(strings.HasPrefix(p, "media/") && WrittenAsMedia(l.Path)):
		if _, err := os.Stat(pathutils.ToMediaPath("media/" + p)); err == nil {
			return "media/" + p
		}
		return pathutils.DocsPath(p)
	case strings.HasPrefix(p, "media/") || strings.HasPrefix(p, "docs/"):
		return pathutils.ToWithPrefix(p)
	}
	if _, err := os.Stat(pathutils.ToMediaPath("media/" + p)); err == nil {
		return "media/" + p
	}
	return pathutils.DocsPath(p)
}

// WrittenAsMedia reports whether the link path p reads media/... once its leading "/", "./" and
// "../" are dropped - a media link, not a link from a doc folder that is called media (a docs
// file in docs/media/ is linked with its /files/ url).
func WrittenAsMedia(p string) bool {
	for {
		switch {
		case strings.HasPrefix(p, "/"):
			p = p[1:]
		case strings.HasPrefix(p, "./"):
			p = p[2:]
		case strings.HasPrefix(p, "../"):
			p = p[3:]
		default:
			return strings.HasPrefix(p, "media/")
		}
	}
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

// tableRows is the byte ranges [start, end) of the rows of the GFM tables in a content.
type tableRows [][2]int

var tableDelimiterRe = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(?:\|\s*:?-+:?\s*)*\|?\s*$`)

// newTableRows finds the tables of content: a header row with a "|", a delimiter row (---|:-:) and
// every following line up to a blank line.
func newTableRows(content string) tableRows {
	lines := strings.Split(content, "\n")
	starts := make([]int, len(lines)+1)
	for i, line := range lines {
		starts[i+1] = starts[i] + len(line) + 1
	}
	var rows tableRows
	for i := 1; i < len(lines); i++ {
		if !strings.Contains(lines[i-1], "|") || !strings.Contains(lines[i], "|") && !strings.Contains(lines[i], "-") || !tableDelimiterRe.MatchString(lines[i]) || strings.TrimSpace(lines[i-1]) == "" {
			continue
		}
		if !strings.Contains(lines[i], "|") && !strings.Contains(lines[i-1], "|") {
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) != "" {
			j++
		}
		rows = append(rows, [2]int{starts[i-1], starts[j]})
		i = j
	}
	return rows
}

func (t tableRows) contains(offset int) bool {
	for _, r := range t {
		if offset >= r[0] && offset < r[1] {
			return true
		}
	}
	return false
}
