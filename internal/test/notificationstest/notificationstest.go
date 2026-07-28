// Package notificationstest - Notifications suite: exercises internal/notificationStorage's
// exported API directly (Add/ConsumePending/GetRecent/DeleteByID/Clear). Notifications are a
// real global sqlite-backed log, not tied to any docs/test/ folder, so cases self-clean the
// same way chattest's global messages do rather than relying on a folder wipe.
package notificationstest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the notification test cases against the real notification storage backend.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("notification-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "notification" }

func (Suite) Run() (*test.SuiteResult, error) {
	cases := []func() test.CaseResult{
		caseFlashConsumedOnce,
		casePersistentList,
		caseDeleteOne,
		caseClearAll,
	}

	result := &test.SuiteResult{Suite: "notification"}
	for _, c := range cases {
		cr := c()
		result.Cases = append(result.Cases, cr)
		if cr.Success {
			result.Passed++
		} else {
			result.Failed++
		}
	}
	result.Total = len(cases)
	result.Success = result.Failed == 0
	return result, nil
}
