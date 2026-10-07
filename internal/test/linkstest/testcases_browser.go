package linkstest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"knov/internal/pathutils"
	"knov/internal/server"
	"knov/internal/test"
	"knov/internal/testkit"

	"github.com/chromedp/chromedp"
)

// pickJS types text into the textarea, fires the keyup the real wiki-autocomplete.js listens to and
// returns "" until the dropdown offers value - then picks it like a click and returns the
// textarea content. %[1]s is the text, %[2]s the value (json strings), %[3]v whether to type.
const pickJS = `(() => {
	const ta = document.querySelector('textarea');
	if (%[3]v) {
		ta.focus(); ta.value = %[1]s; ta.setSelectionRange(ta.value.length, ta.value.length);
		ta.dispatchEvent(new KeyboardEvent('keyup', {key: 'a', bubbles: true}));
		return '';
	}
	const li = Array.from(document.querySelectorAll('#component-autocomplete .autocomplete-item')).find(li => li.getAttribute('data-value') === %[2]s);
	if (!li) return '';
	li.dispatchEvent(new MouseEvent('mousedown', {bubbles: true}));
	return ta.value;
})()`

// caseAutocomplete runs the real wiki-autocomplete.js against the real autocomplete apis in a
// headless browser: for each corpus doc and media file it types the "[[", "](" or "![](" trigger
// plus the folder, picks the file from the dropdown and saves the inserted links in one doc -
// each has to read back as the picked file. Skips if no local Chrome/Chromium is available.
func caseAutocomplete() test.CaseResult {
	name := "links-autocomplete"
	if !testkit.Available() {
		return test.SkipCase(name, "no local chrome/chromium binary found")
	}

	router := server.NewRouter()
	page := `<!DOCTYPE html><html><head><script src="/static/wiki-autocomplete.js"></script></head>
<body><div id="c"><textarea></textarea></div>
<script>initWikiAutocompleteForInputs(document.getElementById('c'), {}, 'textarea');</script></body></html>`
	// one origin, so the script's relative api fetches reach the router
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/links-harness" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(page))
			return
		}
		router.ServeHTTP(w, r)
	}))
	defer ts.Close()

	ctx, cancel, err := testkit.NewBrowser(context.Background())
	if err != nil {
		return test.SkipCase(name, err.Error())
	}
	defer cancel()
	if err := chromedp.Run(ctx, chromedp.Navigate(ts.URL+"/links-harness"), chromedp.WaitReady("textarea", chromedp.ByQuery)); err != nil {
		return errCase(name, err)
	}

	// the inserted links and their targets per trigger, each trigger saved into its own doc
	var gaps []string
	lines, want := map[string][]string{}, map[string][]string{}
	for i := range names {
		// only up to the corpus folder - a ")" in a nested name would end the "](" trigger
		docDir, mediaDir := fmt.Sprintf("%s/%s/c%02d/", testDir, targetsFolder, i), fmt.Sprintf("%s/c%02d/", testDir, i)
		for _, p := range []struct{ kind, text, value, want string }{
			{"wiki", "[[" + docDir, target(i), pathutils.ToWithPrefix(target(i))},
			{"markdown", "[x](" + docDir, target(i), pathutils.ToWithPrefix(target(i))},
			{"media", "![x](" + mediaDir, mediaTarget(i), "media/" + mediaTarget(i)},
		} {
			text, _ := json.Marshal(p.text)
			value, _ := json.Marshal(p.value)
			var out string
			err := chromedp.Run(ctx,
				chromedp.Evaluate(fmt.Sprintf(pickJS, text, value, true), &out),
				chromedp.Poll(fmt.Sprintf(pickJS, text, value, false), &out, chromedp.WithPollingTimeout(3*time.Second)),
			)
			if err != nil || out == "" {
				gaps = append(gaps, fmt.Sprintf("%q: not offered after typing %q (%v)", p.value, p.text, err))
				continue
			}
			lines[p.kind] = append(lines[p.kind], out)
			want[p.kind] = append(want[p.kind], p.want)
		}
	}

	for kind, kindLines := range lines {
		src := testDir + "/src-autocomplete-" + kind + ".md"
		if err := saveDoc(src, strings.Join(kindLines, "\n\n")+"\n"); err != nil {
			return errCase(name, err)
		}
		if kindGaps := linkGaps(src, want[kind]); len(kindGaps) > 0 {
			gaps = append(gaps, kind+" links inserted: "+strings.Join(kindLines, " "))
			for _, g := range kindGaps {
				gaps = append(gaps, kind+" link: "+g)
			}
		}
	}
	return gapsCase(name, "every special-char doc and media file picked from the autocomplete reads back as the picked file", gaps)
}
