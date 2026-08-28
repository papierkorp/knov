package backuptest

import (
	"fmt"
	"strings"
	"time"

	"knov/internal/backup"
	"knov/internal/test"
)

// caseRotate covers backup.Rotate(target, keepDays, keepDefault) against a target seeded with
// synthetic past-dated set names (no real backup content needed - Rotate only inspects names
// and the lock marker): a default set older than both keepDays and outside the keepDefault floor
// is deleted; a partial set outside keepDays is deleted even at an age a default set would have
// survived at via the floor rule; a locked set survives regardless of age.
func caseRotate() test.CaseResult {
	name := "rotate"

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	now := time.Now()
	const keepDays, keepDefault = 7, 2

	// default sets, newest first: dFresh kept via keepDays, dOld1 kept via the keepDefault floor
	// (2nd most recent default set), dOld2 falls outside both and is deleted.
	dFresh := setNameAt(now.Add(-1 * 24 * time.Hour))
	dOld1 := setNameAt(now.Add(-100 * 24 * time.Hour))
	dOld2 := setNameAt(now.Add(-200 * 24 * time.Hour))
	// partial sets get no floor of their own: pFresh survives via keepDays, pOld is deleted at
	// the same age dOld1 (a default set) survives at - the exact bug ParseSetTime/Rotate already
	// caught once (see docs/temp_todo.md).
	pFresh := setNameAt(now.Add(-2*24*time.Hour)) + "_metadata"
	pOld := setNameAt(now.Add(-100*24*time.Hour)) + "_metadata"
	// locked survives despite being older than every rule would otherwise allow.
	locked := setNameAt(now.Add(-300 * 24 * time.Hour))

	for _, n := range []string{dFresh, dOld1, dOld2, pFresh, pOld, locked} {
		if err := target.Write(n, strings.NewReader("")); err != nil {
			return errCase(name, err)
		}
	}
	if err := target.Lock(locked); err != nil {
		return errCase(name, err)
	}

	if err := backup.Rotate(target, keepDays, keepDefault); err != nil {
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

	wantKept := []string{dFresh, dOld1, pFresh, locked}
	wantDeleted := []string{dOld2, pOld}

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
		cr.Error = "Rotate did not apply the locked/keepDays/keepDefault rules as expected, including partial sets getting no long-term floor"
	}
	return cr
}
