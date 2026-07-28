// Package notificationstest - probe-message helpers
package notificationstest

import (
	"knov/internal/notificationStorage"
	"knov/internal/test"
)

const probeLevel = "info"

// drainPending consumes and discards every currently pending notification, so a case
// starting from a known-empty pending queue doesn't observe a stale item left over from
// real app usage or a previous run. This is destructive to any real pending flash message
// that hasn't been displayed yet - an accepted tradeoff for running this suite against a
// live dev instance, same category as jobstest's real cache-invalidate/media-cleanup calls.
func drainPending() error {
	for {
		n, err := notificationStorage.ConsumePending()
		if err != nil {
			return err
		}
		if n == nil {
			return nil
		}
	}
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
