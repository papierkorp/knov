package parser

import (
	"bytes"
	"fmt"
	"html"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// highlightStyleName is the chroma style shared by the web view's HTML/CSS
// highlighting and the pdf export's direct-color token highlighting, so both
// render code identically.
const highlightStyleName = "monokai"

func highlightStyle() *chroma.Style {
	if style := styles.Get(highlightStyleName); style != nil {
		return style
	}
	return styles.Fallback
}

// HighlightToken is a single lexical run of code with the color chroma
// resolves for it under the shared highlighting style. Used by renderers
// (e.g. the pdf export) that draw text directly instead of consuming
// HighlightCode's HTML/CSS output.
type HighlightToken struct {
	Text         string
	Color        [3]int
	Bold, Italic bool
}

// HighlightBackground returns the background color of the shared
// highlighting style, in RGB.
func HighlightBackground() [3]int {
	bg := highlightStyle().Get(chroma.Background).Background
	return [3]int{int(bg.Red()), int(bg.Green()), int(bg.Blue())}
}

// HighlightTokens tokenises code and resolves each token's color under the
// shared highlighting style.
func HighlightTokens(code, language string) []HighlightToken {
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	style := highlightStyle()
	defaultColor := style.Get(chroma.Text).Colour

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return []HighlightToken{{Text: code}}
	}

	var toks []HighlightToken
	for _, tok := range iterator.Tokens() {
		entry := style.Get(tok.Type)
		colour := entry.Colour
		if !colour.IsSet() {
			colour = defaultColor
		}
		toks = append(toks, HighlightToken{
			Text:   tok.Value,
			Color:  [3]int{int(colour.Red()), int(colour.Green()), int(colour.Blue())},
			Bold:   entry.Bold == chroma.Yes,
			Italic: entry.Italic == chroma.Yes,
		})
	}
	return toks
}

// HighlightCode highlights code with the given language
// Returns HTML string with syntax highlighting classes
func HighlightCode(code, language string) string {
	// get lexer for language
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// use built-in style
	style := highlightStyle()

	// create formatter with classes (no inline styles)
	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.ClassPrefix("chroma-"),
		chromahtml.PreventSurroundingPre(true),
	)

	// tokenize and format
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return fmt.Sprintf("<pre class=\"chroma\"><code>%s</code></pre>", html.EscapeString(code))
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return fmt.Sprintf("<pre class=\"chroma\"><code>%s</code></pre>", html.EscapeString(code))
	}

	return buf.String()
}

// HighlightCodeBlock ensures code is properly wrapped in a single pre block
func HighlightCodeBlock(code, language string) string {
	highlighted := HighlightCode(code, language)
	return fmt.Sprintf(`<pre class="chroma"><code class="language-%s">%s</code></pre>`, language, highlighted)
}
