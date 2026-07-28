package notificationstest

import (
	"fmt"
	"slices"

	"knov/internal/notificationStorage"
	"knov/internal/test"
)

// caseFlashConsumedOnce covers notificationStorage.ConsumePending's "atomically returns and
// clears the oldest pending notification" contract - the pending queue is drained first so a
// stale real pending notification can't be mistaken for this case's own probe.
func caseFlashConsumedOnce() test.CaseResult {
	name := "flash-consumed-once"

	if err := drainPending(); err != nil {
		return errCase(name, err)
	}

	added, err := notificationStorage.Add(probeLevel, "notificationstest flash probe", true)
	if err != nil {
		return errCase(name, err)
	}

	first, err := notificationStorage.ConsumePending()
	if err != nil {
		return errCase(name, err)
	}
	second, err := notificationStorage.ConsumePending()
	if err != nil {
		return errCase(name, err)
	}

	firstOK := first != nil && first.ID == added.ID
	secondOK := second == nil

	success := firstOK && secondOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "first ConsumePending returns the added notification, second returns nil",
		Actual:   fmt.Sprintf("firstMatched=%v secondNil=%v", firstOK, secondOK),
		Success:  success,
	}
	if !success {
		cr.Error = "pending notification was not consumed exactly once"
	}
	return cr
}

// casePersistentList covers Add(pending=false) + GetRecent - a non-pending notification is a
// permanent log entry, not a one-shot flash.
func casePersistentList() test.CaseResult {
	name := "persistent-list"

	added, err := notificationStorage.Add(probeLevel, "notificationstest persistent probe", false)
	if err != nil {
		return errCase(name, err)
	}
	defer notificationStorage.DeleteByID(added.ID)

	recent, err := notificationStorage.GetRecent(50)
	if err != nil {
		return errCase(name, err)
	}

	found := slices.ContainsFunc(recent, func(n notificationStorage.Notification) bool { return n.ID == added.ID })

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("GetRecent includes notification %s", added.ID),
		Actual:   fmt.Sprintf("found=%v (%d recent entries)", found, len(recent)),
		Success:  found,
	}
	if !found {
		cr.Error = "persistent notification did not appear in GetRecent"
	}
	return cr
}

// caseDeleteOne covers DeleteByID removing a single notification without affecting others.
func caseDeleteOne() test.CaseResult {
	name := "delete-one"

	keep, err := notificationStorage.Add(probeLevel, "notificationstest delete-one keep", false)
	if err != nil {
		return errCase(name, err)
	}
	defer notificationStorage.DeleteByID(keep.ID)

	toDelete, err := notificationStorage.Add(probeLevel, "notificationstest delete-one target", false)
	if err != nil {
		return errCase(name, err)
	}

	if err := notificationStorage.DeleteByID(toDelete.ID); err != nil {
		return errCase(name, err)
	}

	recent, err := notificationStorage.GetRecent(50)
	if err != nil {
		return errCase(name, err)
	}

	deletedGone := !slices.ContainsFunc(recent, func(n notificationStorage.Notification) bool { return n.ID == toDelete.ID })
	keptStays := slices.ContainsFunc(recent, func(n notificationStorage.Notification) bool { return n.ID == keep.ID })

	success := deletedGone && keptStays
	cr := test.CaseResult{
		Name:     name,
		Expected: "deleted notification absent, unrelated notification still present",
		Actual:   fmt.Sprintf("deletedGone=%v keptStays=%v", deletedGone, keptStays),
		Success:  success,
	}
	if !success {
		cr.Error = "DeleteByID did not remove exactly the targeted notification"
	}
	return cr
}

// caseClearAll covers Clear() - this is the real "Clear all" admin action and, like it,
// irreversibly wipes the entire notification log (not just this suite's probes). Seeding two
// probes and confirming an empty GetRecent afterward is the same outcome a user clicking the
// real button would observe; there is no snapshot/restore for this one.
func caseClearAll() test.CaseResult {
	name := "clear-all"

	if _, err := notificationStorage.Add(probeLevel, "notificationstest clear-all probe 1", false); err != nil {
		return errCase(name, err)
	}
	if _, err := notificationStorage.Add(probeLevel, "notificationstest clear-all probe 2", false); err != nil {
		return errCase(name, err)
	}

	if err := notificationStorage.Clear(); err != nil {
		return errCase(name, err)
	}

	recent, err := notificationStorage.GetRecent(50)
	if err != nil {
		return errCase(name, err)
	}

	success := len(recent) == 0
	cr := test.CaseResult{
		Name:     name,
		Expected: "GetRecent returns no notifications after Clear",
		Actual:   fmt.Sprintf("%d notifications remain", len(recent)),
		Success:  success,
	}
	if !success {
		cr.Error = "Clear did not remove all notifications"
	}
	return cr
}
