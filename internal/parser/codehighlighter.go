package parser

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// HighlightCode highlights code with the given language
// Returns HTML string with syntax highlighting classes
func HighlightCode(code, language string) string {
	// get lexer for language
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// use built-in style
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	// create formatter with classes (no inline styles)
	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.ClassPrefix("chroma-"),
	)

	// tokenize and format
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return fmt.Sprintf("<pre><code>%s</code></pre>", html.EscapeString(code))
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return fmt.Sprintf("<pre><code>%s</code></pre>", html.EscapeString(code))
	}

	return buf.String()
}

// HighlightCodeBlock wraps highlighted code in pre/code tags if not already wrapped
func HighlightCodeBlock(code, language string) string {
	highlighted := HighlightCode(code, language)

	// if already wrapped in <pre>, return as is
	if strings.HasPrefix(highlighted, "<pre") {
		return highlighted
	}

	// otherwise wrap it
	return fmt.Sprintf(`<pre class="chroma"><code class="language-%s">%s</code></pre>`, language, highlighted)
}
