package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/pathutils"
)

// TempRoot is the knov_temp_test scratch directory (sibling to the executable) that
// PrepareIsolatedStorage sets up and RemoveIsolatedStorage deletes.
func TempRoot() string {
	return filepath.Join(logging.ResolveBaseDir(), "knov_temp_test")
}

// PrepareIsolatedStorage points every storage path at fresh, empty data/storage directories
// under TempRoot, so `knov --start-tests` - run as its own separate process alongside a live
// `knov` - never touches the user's real docs, git history or databases. Settings start from
// their defaults too, so a live setting can't change a suite's result; each suite seeds its own
// sample data. The git remote is cleared so the empty data dir never clones the real remote or pushes test commits to it.
// Must run before any storage backend is initialized (see main.go).
func PrepareIsolatedStorage() error {
	live := configmanager.GetAppConfig()

	tempRoot := TempRoot()
	tempData := filepath.Join(tempRoot, "data")
	tempStorage := filepath.Join(tempRoot, "storage")

	// KNOV_DATA_PATH/KNOV_STORAGE_PATH may be configured as a relative path (resolved against
	// cwd), while tempData/
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

	// tempData/tempStorage are cleared below - refuse to proceed if
	// that would ever reach into a live path (equal to it, containing it, or contained by it -
	// e.g. a misconfigured KNOV_DATA_PATH pointing inside knov_temp_test), rather than risk
	// RemoveAll deleting real data.
	if pathutils.PathContains(tempData, liveData) || pathutils.PathContains(liveData, tempData) ||
		pathutils.PathContains(tempStorage, liveStorage) || pathutils.PathContains(liveStorage, tempStorage) {
		return fmt.Errorf("knov_temp_test path collides with live data/storage path, refusing to run")
	}

	if err := os.RemoveAll(tempData); err != nil {
		return fmt.Errorf("failed to clear knov_temp_test data: %w", err)
	}
	if err := os.RemoveAll(tempStorage); err != nil {
		return fmt.Errorf("failed to clear knov_temp_test storage: %w", err)
	}

	configmanager.SetDataAndStoragePaths(tempData, tempStorage)
	configmanager.SetGitRemoteForTest("")
	// InitAppConfig already ran InitGitRepository against the live data path - init the
	// isolated data dir too so git works before any suite seeds its sample data
	if err := configmanager.InitGitRepository(); err != nil {
		return fmt.Errorf("failed to init git repository in knov_temp_test: %w", err)
	}
	return nil
}

// RemoveIsolatedStorage deletes TempRoot (data, storage and isolated logs) - used by
// `knov --start-tests --remove` to leave nothing behind after a headless run.
func RemoveIsolatedStorage() error {
	return os.RemoveAll(TempRoot())
}

// RunAllTestsAndLog runs the named suite, or every registered suite if name is empty (see
// RunAllTests), and logs the aggregated pass/fail summary to logging.KeyInAppTests - used by
// `knov --start-tests` once every storage backend is initialized against the isolated storage
// PrepareIsolatedStorage already switched to.
func RunAllTestsAndLog(name string) (*SuiteResult, error) {
	result, err := RunAllTests(name)
	if err != nil {
		return nil, err
	}

	if result.Failed == 0 {
		logging.LogInfo(logging.KeyInAppTests, "run-tests %s: %d passed, %d skipped, %d failed", result.Suite, result.Passed, result.Skipped, result.Failed)
		return result, nil
	}

	var failedNames []string
	for _, c := range result.Cases {
		if !c.Success {
			failedNames = append(failedNames, fmt.Sprintf("%s: %s", c.Name, c.Error))
		}
	}
	logging.LogWarning(logging.KeyInAppTests, "run-tests %s: %d passed, %d skipped, %d failed (%s)", result.Suite, result.Passed, result.Skipped, result.Failed, strings.Join(failedNames, ", "))
	return result, nil
}
