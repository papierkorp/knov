package backuptest

import (
	"fmt"
	"strings"
	"time"

	"knov/internal/backup"
	"knov/internal/test"
)

// caseRotate covers backup.Rotate(target, keepDays, keepFull) against a target seeded with
// synthetic past-dated set names (no real backup content needed - Rotate only inspects names
// and the lock marker): a full set older than both keepDays and outside the keepFull floor is
// deleted; a partial set outside keepDays is deleted even at an age a full set would have
// survived at via the floor rule; a locked set survives regardless of age.
func caseRotate() test.CaseResult {
	name := "rotate"

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	now := time.Now()
	const keepDays, keepFull = 7, 2

	// full sets, newest first: fFresh kept via keepDays, fOld1 kept via the keepFull floor
	// (2nd most recent full set), fOld2 falls outside both and is deleted.
	fFresh := setNameAt(now.Add(-1 * 24 * time.Hour))
	fOld1 := setNameAt(now.Add(-100 * 24 * time.Hour))
	fOld2 := setNameAt(now.Add(-200 * 24 * time.Hour))
	// partial sets get no floor of their own: pFresh survives via keepDays, pOld is deleted at
	// the same age fOld1 (a full set) survives at - the exact bug ParseSetTime/Rotate already
	// caught once (see docs/temp_todo.md).
	pFresh := setNameAt(now.Add(-2*24*time.Hour)) + "_metadata"
	pOld := setNameAt(now.Add(-100*24*time.Hour)) + "_metadata"
	// locked survives despite being older than every rule would otherwise allow.
	locked := setNameAt(now.Add(-300 * 24 * time.Hour))

	for _, n := range []string{fFresh, fOld1, fOld2, pFresh, pOld, locked} {
		if err := target.Write(n, strings.NewReader("")); err != nil {
			return errCase(name, err)
		}
	}
	if err := target.Lock(locked); err != nil {
		return errCase(name, err)
	}

	if err := backup.Rotate(target, keepDays, keepFull); err != nil {
		return errCase(name, err)
	}

	remaining, err := target.List()
	if err != nil {
		return errCase(name, err)
	}
	remainingSet := make(map[string]bool, len(remaining))
	for _, n := range remaining {
		remainingSet[n] = true
	}

	wantKept := []string{fFresh, fOld1, pFresh, locked}
	wantDeleted := []string{fOld2, pOld}

	keptOK := true
	for _, n := range wantKept {
		if !remainingSet[n] {
			keptOK = false
		}
	}
	deletedOK := true
	for _, n := range wantDeleted {
		if remainingSet[n] {
			deletedOK = false
		}
	}

	success := keptOK && deletedOK
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("kept=%v deleted=%v", wantKept, wantDeleted),
		Actual:   fmt.Sprintf("remaining=%v", remaining),
		Success:  success,
	}
	if !success {
		cr.Error = "Rotate did not apply the locked/keepDays/keepFull rules as expected, including partial sets getting no long-term floor"
	}
	return cr
}
