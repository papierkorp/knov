// Package version holds version information: the release version from the
// embedded version.yaml, plus build details injected via -ldflags.
package version

import (
	_ "embed"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed version.yaml
var versionYAML []byte

var (
	// Version is the human-facing release version (from version.yaml).
	Version = ""
	// Build identifies the exact commit a binary was built from
	// (year-commitcount-shorthash), injected via -ldflags.
	Build             = ""
	BuildTime         = ""
	BuildTimeParsed   time.Time
	LastCommitMessage = ""
)

func init() {
	var cfg struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(versionYAML, &cfg); err == nil {
		Version = strings.TrimSpace(cfg.Version)
	}
	if Version == "" {
		Version = "dev"
	}

	if Build == "" {
		year := time.Now().UTC().Format("2006")
		count, err1 := exec.Command("git", "rev-list", "--count", "HEAD").Output()
		hash, err2 := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
		if err1 != nil || err2 != nil {
			Build = "dev"
		} else {
			Build = year + "-" + strings.TrimSpace(string(count)) + "-" + strings.TrimSpace(string(hash)) + "-dev"
		}
	}
	if LastCommitMessage == "" {
		if msg, err := exec.Command("git", "log", "-1", "--pretty=%s").Output(); err == nil {
			LastCommitMessage = strings.TrimSpace(string(msg))
		}
	}
	if BuildTime == "" {
		BuildTimeParsed = time.Now().UTC()
		BuildTime = BuildTimeParsed.Format("2006-01-02 15:04") + " UTC"
	} else {
		t, err := time.ParseInLocation("2006-01-02 15:04 UTC", BuildTime, time.UTC)
		if err == nil {
			BuildTimeParsed = t
		} else {
			BuildTimeParsed = time.Now().UTC()
		}
	}
}
