// Command genchangelog regenerates two things from the git log, run via
// `make changelog`:
//
//   - docs/changelogs/<year>.md  - the full developer history, grouped by month
//     and conventional-commit type (unchanged, kept in the repo, not shown in
//     the app).
//   - docs/releases/<version>.md - curated end-user release notes per git tag
//     (vX.Y.Z), containing only breaking changes, changes, features and fixes.
//     Commits with "[skip changelog]" in the message are left out. A
//     "BREAKING CHANGE:" trailer in the commit body is used as the note text.
//     The oldest release has no previous tag to diff against, so its notes are
//     taken from README.md instead of the full commit history.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	breakingBangRe    = regexp.MustCompile(`^[^:]*!:`)
	typeRe            = regexp.MustCompile(`^([A-Za-z]+)(\([^)]*\))?!?:\s*(.*)$`)
	versionTagRe      = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	breakingTrailerRe = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE:\s*(.*)$`)
)

type section struct {
	title string
	lines []string
}

type monthData struct {
	year     int
	month    time.Month
	breaking []string
	sections map[string]*section
	order    []string
}

func newMonthData(year int, month time.Month) *monthData {
	sections := map[string]*section{
		"change":          {title: "changes"},
		"feat":            {title: "features"},
		"fix":             {title: "fixes"},
		"docs":            {title: "docs"},
		"build/ci/deploy": {title: "deployment"},
		"refactor/chore":  {title: "internal"},
		"style/perf/test": {title: "other"},
	}
	return &monthData{
		year:     year,
		month:    month,
		sections: sections,
		order:    []string{"change", "feat", "fix", "docs", "build/ci/deploy", "refactor/chore", "style/perf/test"},
	}
}

func (m *monthData) empty() bool {
	if len(m.breaking) > 0 {
		return false
	}
	for _, key := range m.order {
		if len(m.sections[key].lines) > 0 {
			return false
		}
	}
	return true
}

type releaseData struct {
	version  string
	commits  int
	breaking []string
	changes  []string
	features []string
	fixes    []string
}

func (r *releaseData) empty() bool {
	return len(r.breaking)+len(r.changes)+len(r.features)+len(r.fixes) == 0
}

func main() {
	releaseVersion := flag.String("version", "", "write the commits after the last tag as this release (vX.Y.Z) instead of unreleased.md - used by `make release`")
	flag.Parse()
	if *releaseVersion != "" && !versionTagRe.MatchString(*releaseVersion) {
		fatal(fmt.Errorf("invalid -version %q, want vX.Y.Z", *releaseVersion))
	}

	repo, err := git.PlainOpen(".")
	if err != nil {
		fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		fatal(err)
	}
	iter, err := repo.Log(&git.LogOptions{From: head.Hash(), Order: git.LogOrderCommitterTime})
	if err != nil {
		fatal(err)
	}

	tags := versionTags(repo)

	months := map[string]*monthData{}
	yearMonths := map[int][]string{}
	var years []int
	seenYear := map[int]bool{}

	releases := map[string]*releaseData{}
	var releaseOrder []string
	currentVersion := "unreleased"
	if *releaseVersion != "" {
		currentVersion = *releaseVersion
	}

	err = iter.ForEach(func(c *object.Commit) error {
		when := c.Author.When
		year, month := when.Year(), when.Month()
		key := fmt.Sprintf("%04d-%02d", year, month)

		md, ok := months[key]
		if !ok {
			md = newMonthData(year, month)
			months[key] = md
			yearMonths[year] = append(yearMonths[year], key)
			if !seenYear[year] {
				seenYear[year] = true
				years = append(years, year)
			}
		}

		subject := strings.SplitN(c.Message, "\n", 2)[0]
		classifyCommit(md, subject)

		if v, ok := tags[c.Hash]; ok {
			currentVersion = v
		}
		rd, ok := releases[currentVersion]
		if !ok {
			rd = &releaseData{version: currentVersion}
			releases[currentVersion] = rd
			releaseOrder = append(releaseOrder, currentVersion)
		}
		rd.commits++
		classifyRelease(rd, subject, c.Message)
		return nil
	})
	if err != nil {
		fatal(err)
	}

	writeChangelogs(years, yearMonths, months)
	writeReleases(releaseOrder, releases, len(tags) > 0)
}

func writeChangelogs(years []int, yearMonths map[int][]string, months map[string]*monthData) {
	if err := os.MkdirAll("docs/changelogs", 0755); err != nil {
		fatal(err)
	}

	for _, year := range years {
		path := fmt.Sprintf("docs/changelogs/%d.md", year)
		var buf strings.Builder
		fmt.Fprintf(&buf, "# changelog %d\n\n", year)

		for _, key := range yearMonths[year] {
			md := months[key]
			if md.empty() {
				continue
			}
			fmt.Fprintf(&buf, "## %s\n\n", md.month.String())

			if len(md.breaking) > 0 {
				writeSection(&buf, "###", "breaking changes", md.breaking)
			}
			for _, sk := range md.order {
				s := md.sections[sk]
				if len(s.lines) > 0 {
					writeSection(&buf, "###", s.title, s.lines)
				}
			}
		}

		if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
			fatal(err)
		}
		fmt.Printf("changelog written to %s\n", path)
	}
}

func writeReleases(order []string, releases map[string]*releaseData, hasTags bool) {
	if err := os.MkdirAll("docs/releases", 0755); err != nil {
		fatal(err)
	}
	// unreleased.md is a draft and is always regenerated; every tagged
	// release's notes are frozen the moment the file is written so later
	// runs (e.g. a README.md edit) never rewrite already-published notes.
	os.Remove("docs/releases/unreleased.md")

	// the oldest release spans back to the repo root and has no previous tag to
	// diff against, so its huge commit list is useless - use README.md instead.
	initial := ""
	for i := len(order) - 1; i >= 0; i-- {
		if order[i] != "unreleased" {
			initial = order[i]
			break
		}
	}

	for _, v := range order {
		rd := releases[v]
		if v == "unreleased" && !hasTags {
			continue
		}
		if v != initial && rd.empty() {
			continue
		}
		path := "docs/releases/" + v + ".md"
		if v != "unreleased" {
			if _, err := os.Stat(path); err == nil {
				continue
			}
		}
		var buf strings.Builder
		fmt.Fprintf(&buf, "# %s\n\n", v)
		if v == initial {
			readme, err := os.ReadFile("README.md")
			if err != nil {
				fatal(err)
			}
			buf.Write(readme)
		} else {
			fmt.Fprintf(&buf, "_%d commits since last release_\n\n", rd.commits)
			writeSection(&buf, "##", "breaking changes", rd.breaking)
			writeSection(&buf, "##", "changes", rd.changes)
			writeSection(&buf, "##", "features", rd.features)
			writeSection(&buf, "##", "fixes", rd.fixes)
		}

		if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
			fatal(err)
		}
		fmt.Printf("release notes written to %s\n", path)
	}
}

// versionTags maps the commit hash of every vX.Y.Z tag (lightweight or
// annotated) to its tag name.
func versionTags(repo *git.Repository) map[plumbing.Hash]string {
	out := map[plumbing.Hash]string{}
	iter, err := repo.Tags()
	if err != nil {
		return out
	}
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if !versionTagRe.MatchString(name) {
			return nil
		}
		if to, err := repo.TagObject(ref.Hash()); err == nil {
			if c, err := to.Commit(); err == nil {
				out[c.Hash] = name
			}
			return nil
		}
		out[ref.Hash()] = name
		return nil
	})
	return out
}

func classifyCommit(md *monthData, subject string) {
	isBreaking := breakingBangRe.MatchString(subject) || strings.HasPrefix(subject, "BREAKING CHANGE:")
	if isBreaking {
		md.breaking = append(md.breaking, subject)
	}

	match := typeRe.FindStringSubmatch(subject)
	if match == nil {
		return
	}
	commitType, desc := strings.ToLower(match[1]), match[3]
	line := "- " + desc

	switch commitType {
	case "feat":
		md.sections["feat"].lines = append(md.sections["feat"].lines, line)
	case "fix":
		md.sections["fix"].lines = append(md.sections["fix"].lines, line)
	case "change":
		md.sections["change"].lines = append(md.sections["change"].lines, line)
	case "docs":
		md.sections["docs"].lines = append(md.sections["docs"].lines, line)
	case "build", "ci", "deploy":
		md.sections["build/ci/deploy"].lines = append(md.sections["build/ci/deploy"].lines, line)
	case "refactor", "chore":
		md.sections["refactor/chore"].lines = append(md.sections["refactor/chore"].lines, line)
	case "style", "perf", "test":
		md.sections["style/perf/test"].lines = append(md.sections["style/perf/test"].lines, line)
	}
}

// classifyRelease adds a commit to the curated release notes. Breaking commits
// go only under "breaking changes"; everything else is limited to the
// change/feat/fix types so internal work never reaches end users.
func classifyRelease(rd *releaseData, subject, message string) {
	if strings.Contains(message, "[skip changelog]") {
		return
	}
	if note := breakingNote(subject, message); note != "" {
		rd.breaking = append(rd.breaking, "- "+note)
		return
	}
	match := typeRe.FindStringSubmatch(subject)
	if match == nil {
		return
	}
	switch strings.ToLower(match[1]) {
	case "change":
		rd.changes = append(rd.changes, "- "+match[3])
	case "feat":
		rd.features = append(rd.features, "- "+match[3])
	case "fix":
		rd.fixes = append(rd.fixes, "- "+match[3])
	}
}

// breakingNote returns the upgrade note for a breaking commit, or "" when the
// commit is not breaking. A "BREAKING CHANGE:" body trailer wins over the
// subject; otherwise a "type!:" subject falls back to its description.
func breakingNote(subject, message string) string {
	if m := breakingTrailerRe.FindStringSubmatch(message); m != nil && strings.TrimSpace(m[1]) != "" {
		return strings.TrimSpace(m[1])
	}
	if breakingBangRe.MatchString(subject) {
		if m := typeRe.FindStringSubmatch(subject); m != nil {
			return m[3]
		}
	}
	return ""
}

func writeSection(buf *strings.Builder, prefix, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(buf, "%s %s\n", prefix, title)
	for _, l := range lines {
		fmt.Fprintln(buf, l)
	}
	fmt.Fprintln(buf)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "genchangelog: %v\n", err)
	os.Exit(1)
}
