// Command genchangelog regenerates docs/changelogs/<year>.md from the git log,
// grouped by month and conventional-commit type. Run via `make changelog`.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	breakingBangRe = regexp.MustCompile(`^[^:]*!:`)
	typeRe         = regexp.MustCompile(`^([A-Za-z]+)(\([^)]*\))?!?:\s*(.*)$`)
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
		order:    []string{"feat", "fix", "docs", "build/ci/deploy", "refactor/chore", "style/perf/test"},
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

func main() {
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

	months := map[string]*monthData{}
	yearMonths := map[int][]string{}
	var years []int
	seenYear := map[int]bool{}

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
		return nil
	})
	if err != nil {
		fatal(err)
	}

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
				writeSection(&buf, "breaking changes", md.breaking)
			}
			for _, sk := range md.order {
				s := md.sections[sk]
				if len(s.lines) > 0 {
					writeSection(&buf, s.title, s.lines)
				}
			}
		}

		if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
			fatal(err)
		}
		fmt.Printf("changelog written to %s\n", path)
	}
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

func writeSection(buf *strings.Builder, title string, lines []string) {
	fmt.Fprintf(buf, "### %s\n", title)
	for _, l := range lines {
		fmt.Fprintln(buf, l)
	}
	fmt.Fprintln(buf)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "genchangelog: %v\n", err)
	os.Exit(1)
}
