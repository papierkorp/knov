// Package version holds build-time version information injected via -ldflags.
package version

import (
	"os/exec"
	"strings"
	"time"
)

var (
	Version           = ""
	BuildTime         = ""
	BuildTimeParsed   time.Time
	LastCommitMessage = ""
)

func init() {
	if Version == "" {
		year := time.Now().UTC().Format("2006")
		count, err1 := exec.Command("git", "rev-list", "--count", "HEAD").Output()
		hash, err2 := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
		if err1 != nil || err2 != nil {
			Version = "dev"
		} else {
			Version = year + "-" + strings.TrimSpace(string(count)) + "-" + strings.TrimSpace(string(hash)) + "-dev"
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
