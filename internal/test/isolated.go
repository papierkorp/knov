package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/backup"
	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/pathutils"
)

// TempRoot is the knov_temp_test scratch directory (sibling to the executable) that
// PrepareIsolatedStorage copies into and RemoveIsolatedStorage deletes.
func TempRoot() string {
	return filepath.Join(logging.ResolveBaseDir(), "knov_temp_test")
}

// PrepareIsolatedStorage points every storage path at a fresh copy of the live data/storage
// directories under TempRoot, so `knov --start-tests` - run as its own separate process
// alongside a live `knov` - never touches the user's real docs, git history or databases.
// Must run before any storage backend is initialized (see main.go).
func PrepareIsolatedStorage() error {
	live := configmanager.GetAppConfig()

	tempRoot := TempRoot()
	tempData := filepath.Join(tempRoot, "data")
	tempStorage := filepath.Join(tempRoot, "storage")

	// KNOV_DATA_PATH/KNOV_STORAGE_PATH may be configured as a relative path (resolved against
	// cwd, same as backup.RestoreFile's own os.Stat/WalkDir calls below), while tempData/
	// tempStorage are always absolute - resolve both sides the same way before comparing, or a
	// relative live path could never match tempData/tempStorage and slip past the check below.
	liveData, err := filepath.Abs(live.DataPath)
	if err != nil {
		return fmt.Errorf("failed to resolve data path: %w", err)
	}
	liveStorage, err := filepath.Abs(live.StoragePath)
	if err != nil {
		return fmt.Errorf("failed to resolve storage path: %w", err)
	}

	// backup.RestoreFile clears its destination before copying into it - refuse to proceed if
	// that would ever reach into a live path (equal to it, containing it, or contained by it -
	// e.g. a misconfigured KNOV_DATA_PATH pointing inside knov_temp_test), rather than risk
	// RemoveAll deleting real data.
	if pathutils.PathContains(tempData, liveData) || pathutils.PathContains(liveData, tempData) ||
		pathutils.PathContains(tempStorage, liveStorage) || pathutils.PathContains(liveStorage, tempStorage) {
		return fmt.Errorf("knov_temp_test path collides with live data/storage path, refusing to run")
	}

	if err := backup.RestoreFile(live.DataPath, tempData); err != nil {
		return fmt.Errorf("failed to copy data into knov_temp_test: %w", err)
	}
	if err := backup.RestoreFile(live.StoragePath, tempStorage); err != nil {
		return fmt.Errorf("failed to copy storage into knov_temp_test: %w", err)
	}

	configmanager.SetDataAndStoragePaths(tempData, tempStorage)
	return nil
}

// RemoveIsolatedStorage deletes TempRoot (data, storage and isolated logs) - used by
// `knov --start-tests --remove` to leave nothing behind after a headless run.
func RemoveIsolatedStorage() error {
	return os.RemoveAll(TempRoot())
}

// RunAllTestsAndLog runs every registered suite (see RunAllTests) and logs the aggregated
// pass/fail summary to logging.KeyInAppTests - used by `knov --start-tests` once every storage
// backend is initialized against the isolated copy PrepareIsolatedStorage already switched to.
func RunAllTestsAndLog() (*SuiteResult, error) {
	result, err := RunAllTests()
	if err != nil {
		return nil, err
	}

	if result.Failed == 0 {
		logging.LogInfo(logging.KeyInAppTests, "run-all-tests: %d passed, %d failed", result.Passed, result.Failed)
		return result, nil
	}

	var failedNames []string
	for _, c := range result.Cases {
		if !c.Success {
			failedNames = append(failedNames, fmt.Sprintf("%s: %s", c.Name, c.Error))
		}
	}
	logging.LogWarning(logging.KeyInAppTests, "run-all-tests: %d passed, %d failed (%s)", result.Passed, result.Failed, strings.Join(failedNames, ", "))
	return result, nil
}
