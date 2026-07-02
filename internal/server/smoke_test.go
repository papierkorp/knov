package server_test

// Smoke tests confirming testkit boots the real router and that headless
// chromedp can drive pages from it in this environment. Actual
// tier-1/tier-2 test coverage lives in dedicated *_test.go files per
// api_*.go handler; see docs/testing.md and docs/temp_todo.md.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"knov/internal/testkit"

	"github.com/chromedp/chromedp"
)

func TestTestkitSmoke(t *testing.T) {
	ts := testkit.NewApp(t)

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestChromedpSmoke(t *testing.T) {
	ts := testkit.NewApp(t)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 15*time.Second)
	defer cancelTimeout()

	var body string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL+"/"),
		chromedp.OuterHTML("html", &body),
	); err != nil {
		t.Fatalf("chromedp run: %v", err)
	}

	if !strings.Contains(body, "<html") {
		t.Fatalf("expected rendered html, got: %.200s", body)
	}
}
