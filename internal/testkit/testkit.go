// Package testkit provides headless-browser (chromedp) helpers for the handful of
// interactions an internal/test/<x>test suite can't verify by calling Go functions
// directly - real DOM/JS behavior like native HTML5 drag-and-drop or a JS library's
// undo/redo module. See docs/testing.md.
package testkit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/chromedp/chromedp"
)

// ErrNoBrowser is returned by NewBrowser when no local Chrome/Chromium binary was found,
// so callers can skip the case instead of failing on machines that never installed one.
var ErrNoBrowser = errors.New("no chrome/chromium binary found")

// browserTimeout bounds every chromedp session started via NewBrowser, so a page/selector
// that never resolves (a bug, or a CDP/browser hang) fails the case instead of hanging
// `knov --start-tests` forever.
const browserTimeout = 30 * time.Second

var browserNames = []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"}

// Available reports whether a local Chrome/Chromium binary chromedp can drive was found.
// On Windows, exec.LookPath alone isn't enough since Chrome's installer doesn't add itself
// to PATH, so the common per-user/per-machine install locations are also checked.
func Available() bool {
	for _, name := range browserNames {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	if runtime.GOOS != "windows" {
		return false
	}
	for _, dir := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")} {
		if dir == "" {
			continue
		}
		for _, rel := range []string{`Google\Chrome\Application\chrome.exe`, `Chromium\Application\chrome.exe`} {
			if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
				return true
			}
		}
	}
	return false
}

// NewBrowser starts a headless chromedp context. The caller must call the returned
// cancel func (defer it) once done.
func NewBrowser(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if !Available() {
		return nil, nil, ErrNoBrowser
	}
	opts := chromedp.DefaultExecAllocatorOptions[:]
	// Chrome's sandbox refuses to start when the calling process is root, which is the
	// common case in containerized CI - without this it would hang until browserTimeout
	// and look like a genuine test failure rather than an environment quirk.
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		opts = append(opts, chromedp.NoSandbox)
	}
	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, browserTimeout)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(timeoutCtx, opts...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	cancel := func() { cancelBrowser(); cancelAlloc(); cancelTimeout() }
	return browserCtx, cancel, nil
}

// Drag simulates a native HTML5 drag-and-drop from sourceSelector to targetSelector by
// dispatching real dragstart/dragover/drop/dragend DragEvents with a DataTransfer object -
// headless Chrome has no OS-level mouse to drive an actual browser drag, and this is the
// standard way to exercise dragstart/dragover/drop handlers without one.
func Drag(sourceSelector, targetSelector string) chromedp.Action {
	return chromedp.Evaluate(fmt.Sprintf(`(function() {
		var source = document.querySelector(%q);
		var target = document.querySelector(%q);
		var dt = new DataTransfer();
		var opts = { bubbles: true, cancelable: true, dataTransfer: dt };
		source.dispatchEvent(new DragEvent('dragstart', opts));
		target.dispatchEvent(new DragEvent('dragover', opts));
		target.dispatchEvent(new DragEvent('drop', opts));
		document.dispatchEvent(new DragEvent('dragend', opts));
	})()`, sourceSelector, targetSelector), nil)
}
