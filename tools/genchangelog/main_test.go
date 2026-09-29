package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestUpgradeNotes(t *testing.T) {
	cases := []struct {
		name, in, want string
		wantErr        bool
	}{
		{name: "template only is empty", in: "<!-- template -->\n", want: ""},
		{name: "headings are shifted", in: "<!-- t -->\n## removed\n- x", want: "### removed\n- x"},
		{name: "tags and fenced lines are kept", in: "#kanban\n```\n## code\n```", want: "#kanban\n```\n## code\n```"},
		{name: "top-level heading is rejected", in: "# title", wantErr: true},
		{name: "six-level heading is rejected", in: "###### deep", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := upgradeNotes(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// setupRelease creates a repo layout with a frozen v1.0.0 release and the
// given upgrade notes, returning the order/releases for cutting v1.1.0.
func setupRelease(t *testing.T, upgrade string, rd *releaseData) ([]string, map[string]*releaseData) {
	t.Chdir(t.TempDir())
	os.MkdirAll("docs/releases", 0755)
	os.WriteFile("README.md", []byte("readme"), 0644)
	os.WriteFile("docs/releases/v1.0.0.md", []byte("frozen"), 0644)
	os.WriteFile(upgradeFile, []byte(upgrade), 0644)
	return []string{"v1.1.0", "v1.0.0"}, map[string]*releaseData{"v1.1.0": rd, "v1.0.0": {version: "v1.0.0"}}
}

func TestWriteReleasesMovesUpgradeNotes(t *testing.T) {
	order, releases := setupRelease(t, "<!-- t -->\n## removed\n- x\n", &releaseData{commits: 2, breaking: []string{"- b"}, fixes: []string{"- f"}})
	writeReleases(order, releases, true, "v1.1.0")

	got, _ := os.ReadFile("docs/releases/v1.1.0.md")
	if want := "# v1.1.0\n\n_2 commits since last release_\n\n## upgrading from v1.0.0 to v1.1.0\n\n### removed\n- x\n\n## fixes\n- f\n\n"; string(got) != want {
		t.Errorf("release notes = %q, want %q", got, want)
	}
	if got, _ := os.ReadFile(upgradeFile); string(got) != "<!-- t -->\n" {
		t.Errorf("upgrade file = %q, want reset to template", got)
	}
	if got, _ := os.ReadFile("docs/releases/v1.0.0.md"); string(got) != "frozen" {
		t.Errorf("frozen release was rewritten: %q", got)
	}
}

// fatal exits, so the blocked release runs in a subprocess
func TestWriteReleasesBlocksBreakingWithoutNotes(t *testing.T) {
	if os.Getenv("GENCHANGELOG_BLOCK") == "1" {
		order, releases := setupRelease(t, "<!-- t -->\n", &releaseData{commits: 1, breaking: []string{"- b"}})
		writeReleases(order, releases, true, "v1.1.0")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestWriteReleasesBlocksBreakingWithoutNotes$")
	cmd.Env = append(os.Environ(), "GENCHANGELOG_BLOCK=1")
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "is empty") {
		t.Errorf("release with breaking commits and no upgrade notes should fail, got err=%v out=%s", err, out)
	}
}
