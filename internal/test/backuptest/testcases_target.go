package backuptest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/backup"
	"knov/internal/test"
	"knov/internal/utils"
)

// caseEventRoundtripAcrossRestart covers backup.Event round-tripping through LogEvent/Events on
// a fresh localTarget, including across a simulated restart (a new NewLocalTarget instance
// pointed at the same root, mirroring how the real app re-opens its target after every restart).
func caseEventRoundtripAcrossRestart() test.CaseResult {
	name := "event-roundtrip-across-restart"

	dir, err := os.MkdirTemp("", "knov-backuptest-events-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(dir)

	target, err := backup.NewLocalTarget(dir)
	if err != nil {
		return errCase(name, err)
	}
	if err := target.LogEvent(backup.EventBackup, "set-a", backup.SourceManual); err != nil {
		return errCase(name, err)
	}
	if err := target.LogEvent(backup.EventRestore, "set-a", backup.SourceRestore); err != nil {
		return errCase(name, err)
	}

	restarted, err := backup.NewLocalTarget(dir)
	if err != nil {
		return errCase(name, err)
	}
	events, err := restarted.Events()
	if err != nil {
		return errCase(name, err)
	}

	ok := len(events) == 2 &&
		events[0].Kind == backup.EventBackup && events[0].Set == "set-a" && events[0].Source == backup.SourceManual &&
		events[1].Kind == backup.EventRestore && events[1].Set == "set-a" && events[1].Source == backup.SourceRestore

	cr := test.CaseResult{
		Name:     name,
		Expected: "2 events (backup, restore) for set-a survive a fresh localTarget instance pointed at the same root",
		Actual:   fmt.Sprintf("events=%+v", events),
		Success:  ok,
	}
	if !ok {
		cr.Error = "Events did not round-trip across a simulated restart as expected"
	}
	return cr
}

// caseWriteFileAtomicNoTornReads covers utils.WriteFileAtomic: a reader racing a concurrent
// write never observes a torn/partial file - only ever the old or the new full content.
func caseWriteFileAtomicNoTornReads() test.CaseResult {
	name := "write-file-atomic-no-torn-reads"

	dir, err := os.MkdirTemp("", "knov-backuptest-atomic-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "probe.txt")

	contentA := bytes.Repeat([]byte("A"), 5000)
	contentB := bytes.Repeat([]byte("B"), 9000)
	if err := utils.WriteFileAtomic(path, contentA, 0644); err != nil {
		return errCase(name, err)
	}

	const iterations = 200
	stop := make(chan struct{})
	var writeErr error
	go func() {
		defer close(stop)
		for i := 0; i < iterations; i++ {
			content := contentA
			if i%2 == 1 {
				content = contentB
			}
			if err := utils.WriteFileAtomic(path, content, 0644); err != nil {
				writeErr = err
				return
			}
		}
	}()

	var readErr error
	torn := false
reads:
	for {
		select {
		case <-stop:
			break reads
		default:
		}
		data, err := os.ReadFile(path)
		if err != nil {
			// windows refuses to open a file for a moment while it is replaced - nothing torn was read
			if utils.IsSharingViolation(err) {
				continue
			}
			if !os.IsNotExist(err) {
				readErr = err
				break reads
			}
			continue
		}
		if !bytes.Equal(data, contentA) && !bytes.Equal(data, contentB) {
			torn = true
			break reads
		}
	}

	success := writeErr == nil && readErr == nil && !torn
	cr := test.CaseResult{
		Name:     name,
		Expected: "a reader racing WriteFileAtomic always observes either the old or the new full content, never a torn mix",
		Actual:   fmt.Sprintf("torn=%v writeErr=%v readErr=%v", torn, writeErr, readErr),
		Success:  success,
	}
	if !success {
		cr.Error = "WriteFileAtomic did not prevent a torn read as expected"
	}
	return cr
}
