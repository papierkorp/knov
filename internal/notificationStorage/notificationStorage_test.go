package notificationStorage

import (
	"os"
	"slices"
	"testing"
)

const probeLevel = "info"

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "knov-notificationStorage-test")
	if err != nil {
		panic(err)
	}
	if err := Init(dir); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// drainPending consumes and discards every currently pending notification, so a test starting
// from a known-empty pending queue doesn't observe a stale item left over from an earlier test
// in this package.
func drainPending(t *testing.T) {
	t.Helper()
	for {
		n, err := ConsumePending()
		if err != nil {
			t.Fatal(err)
		}
		if n == nil {
			return
		}
	}
}

// ConsumePending's "atomically returns and clears the oldest pending notification" contract.
func TestFlashConsumedOnce(t *testing.T) {
	drainPending(t)

	added, err := Add(probeLevel, "notificationStorage test flash probe", true)
	if err != nil {
		t.Fatal(err)
	}

	first, err := ConsumePending()
	if err != nil {
		t.Fatal(err)
	}
	second, err := ConsumePending()
	if err != nil {
		t.Fatal(err)
	}

	if first == nil || first.ID != added.ID {
		t.Errorf("first ConsumePending = %v, want the added notification", first)
	}
	if second != nil {
		t.Errorf("second ConsumePending = %v, want nil", second)
	}
}

// Add(pending=false) + GetRecent - a non-pending notification is a permanent log entry, not a
// one-shot flash.
func TestPersistentList(t *testing.T) {
	added, err := Add(probeLevel, "notificationStorage test persistent probe", false)
	if err != nil {
		t.Fatal(err)
	}
	defer DeleteByID(added.ID)

	recent, err := GetRecent(50)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(recent, func(n Notification) bool { return n.ID == added.ID }) {
		t.Errorf("GetRecent did not include notification %s", added.ID)
	}
}

// DeleteByID removes a single notification without affecting others.
func TestDeleteOne(t *testing.T) {
	keep, err := Add(probeLevel, "notificationStorage test delete-one keep", false)
	if err != nil {
		t.Fatal(err)
	}
	defer DeleteByID(keep.ID)

	toDelete, err := Add(probeLevel, "notificationStorage test delete-one target", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteByID(toDelete.ID); err != nil {
		t.Fatal(err)
	}

	recent, err := GetRecent(50)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(recent, func(n Notification) bool { return n.ID == toDelete.ID }) {
		t.Error("deleted notification is still present")
	}
	if !slices.ContainsFunc(recent, func(n Notification) bool { return n.ID == keep.ID }) {
		t.Error("unrelated notification was removed")
	}
}

// Clear() wipes the entire notification log.
func TestClearAll(t *testing.T) {
	if _, err := Add(probeLevel, "notificationStorage test clear-all probe 1", false); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(probeLevel, "notificationStorage test clear-all probe 2", false); err != nil {
		t.Fatal(err)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}

	recent, err := GetRecent(50)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 0 {
		t.Errorf("%d notifications remain after Clear, want 0", len(recent))
	}
}
