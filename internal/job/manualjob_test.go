package job

import (
	"context"
	"testing"
)

func TestRepairBrokenLinksJobCountsMalformedEntriesAsSkipped(t *testing.T) {
	j := &repairBrokenLinksJob{entries: []string{"a.md|b.md|c.md", `["a.md"]`, `["a.md","","c.md"]`, "not json"}}
	if err := j.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if j.result.Skipped != 4 || j.result.Repaired != 0 {
		t.Fatalf("expected 4 skipped, got %+v", j.result)
	}
}
