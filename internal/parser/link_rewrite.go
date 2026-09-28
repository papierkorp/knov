package parser

import (
	"regexp"
	"strings"

	"knov/internal/markdown"
	"knov/internal/pathutils/crosspath"
)

// LinkKind is the syntax a link was written in.
type LinkKind int

const (
	LinkMarkdown LinkKind = iota // ](dest) and reference-style [id]: dest
	LinkWiki                     // [[path|text]]
	LinkHTML                     // <img/a/video/audio/source src/href="...">
)

var (
	rewriteMdLinkRe   = regexp.MustCompile(`\]\((<[^>\n]*>[^)\n]*|(?:[^()\n]|\([^()\n]*\))+)\)`)
	rewriteWikiLinkRe = regexp.MustCompile(`\[\[([^\[\]|\n]+)`)
	rewriteHTMLAttrRe = regexp.MustCompile(`(<(?i:img|a|video|audio|source)(?:\s[^>]*?)?\s(?:src|href)\s*=\s*["'])([^"'\n]+)`)
	// [id]: dest, not [^footnote]: - dest is <...> or has no spaces, only a size / title may follow, so prose like "[note]: remember this" isn't a link
	rewriteRefDefRe   = regexp.MustCompile(`^( {0,3}\[[^\]^][^\]]*\]:[ \t]*)((?:<[^>\n]*>|\S+)(?:[ \t]+=\d*x\d*)?(?:[ \t]+(?:"[^"\n]*"|'[^'\n]*'|\([^)\n]*\)))?[ \t\r]*)$`)
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

// RewriteLinks replaces, outside fenced code blocks and inline code, the path of every markdown
// ](dest), [[wiki]], reference-style [id]: dest and html src/href link for which fn returns a new
// path (fn gets the path without angle brackets, title, query or anchor, with windows "\"
// separators as "/" and for markdown with escapes resolved). Anything around the path is kept
// as-is, except a wiki.js " =WxH" image size, which goldmark doesn't parse and would break the
// image. Returns the content and whether anything was replaced.
// ExtractLinks walks links through this too, so both always see the same links.
func RewriteLinks(content string, fn func(p string, kind LinkKind) (string, bool)) (string, bool) {
	changed := false
	replace := func(dest, stops string, kind LinkKind, titled bool) string {
		prefix, p, suffix := splitLinkPath(dest, stops, titled)
		if kind == LinkMarkdown {
			p = markdownLinkPath(p)
		} else {
			p = crosspath.ToSlash(p)
		}
		newPath, ok := fn(p, kind)
		if !ok || newPath == p {
			return dest
		}
		changed = true
		return prefix + newPath + wikijsImageSizeRe.ReplaceAllString(suffix, "")
	}

	lines := strings.Split(content, "\n")
	fenced := markdown.FenceMask(lines)
	for i, line := range lines {
		if fenced[i] {
			continue
		}
		if sub := rewriteRefDefRe.FindStringSubmatch(line); sub != nil {
			lines[i] = sub[1] + replace(sub[2], "?#", LinkMarkdown, true)
			continue
		}
		// odd parts are inline `code` spans
		parts := markdown.SplitCodeSpans(line)
		for j := 0; j < len(parts); j += 2 {
			part := rewriteMdLinkRe.ReplaceAllStringFunc(parts[j], func(m string) string {
				return "](" + replace(m[2:len(m)-1], "?#", LinkMarkdown, true) + ")"
			})
			part = rewriteWikiLinkRe.ReplaceAllStringFunc(part, func(m string) string {
				return "[[" + replace(m[2:], "#", LinkWiki, false)
			})
			parts[j] = rewriteHTMLAttrRe.ReplaceAllStringFunc(part, func(m string) string {
				// the path can't contain quotes, so the last one ends the tag prefix
				i := strings.LastIndexAny(m, `"'`) + 1
				return m[:i] + replace(m[i:], "?#", LinkHTML, false)
			})
		}
		lines[i] = strings.Join(parts, "")
	}
	return strings.Join(lines, "\n"), changed
}

// IsAppRouteLink reports whether a link is an html root link to an app route (/dashboard,
// /search?q=...) rather than a file - for html only /media/ and /files/ are files.
func IsAppRouteLink(p string, kind LinkKind) bool {
	return kind == LinkHTML && strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "/media/") && !strings.HasPrefix(p, "/files/")
}

// splitLinkPath splits a link destination into leading whitespace / "<", the path and the
// untouched rest. The path ends at the first of stops, at ">" for an angle-bracketed
// destination, and - if titled - at a " =WxH" size or "title" (spaces inside the path stay,
// trailing ones and a crlf "\r" go to the rest).
func splitLinkPath(dest, stops string, titled bool) (prefix, p, suffix string) {
	trimmed := strings.TrimLeft(dest, " \t")
	prefix = dest[:len(dest)-len(trimmed)]
	if rest, ok := strings.CutPrefix(trimmed, "<"); ok {
		prefix += "<"
		trimmed, stops, titled = rest, ">?#", false
	}
	end := strings.IndexAny(trimmed, stops)
	if end == -1 {
		end = len(trimmed)
	}
	if loc := linkSuffixRe.FindStringIndex(trimmed); titled && loc != nil && loc[0] < end {
		end = loc[0]
	}
	p = strings.TrimRight(trimmed[:end], " \t\r") // \r: crlf line ending
	return prefix, p, trimmed[len(p):]
}
