// Package main ..
package main

import (
	"embed"
	"fmt"
	"os"
	"time"

	"knov/internal/cacheStorage"
	"knov/internal/chatStorage"
	"knov/internal/cli"
	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/contentHandler"
	"knov/internal/contentStorage"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/fonts"
	"knov/internal/git"
	"knov/internal/job"
	"knov/internal/jobStorage"
	"knov/internal/kanban"
	"knov/internal/kanbanStorage"
	"knov/internal/logging"
	"knov/internal/metadataStorage"
	"knov/internal/notificationStorage"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/pdfexport"
	"knov/internal/searchStorage"
	"knov/internal/server"
	"knov/internal/trackerStorage"

	"knov/internal/test"
	// every in-app test suite self-registers with test.Register in its own init() -
	// these blank imports are what actually trigger that (run via `knov --start-tests`,
	// see runHeadlessTests below).
	_ "knov/internal/test/asyncjobtest"
	_ "knov/internal/test/backuptest"
	_ "knov/internal/test/browsetest"
	_ "knov/internal/test/chattest"
	_ "knov/internal/test/connectionstest"
	_ "knov/internal/test/dashboardtest"
	_ "knov/internal/test/editorstest"
	_ "knov/internal/test/exporttest"
	_ "knov/internal/test/filtertest"
	_ "knov/internal/test/githistorytest"
	_ "knov/internal/test/jobstest"
	_ "knov/internal/test/kanbantest"
	_ "knov/internal/test/linkstest"
	_ "knov/internal/test/logstest"
	_ "knov/internal/test/mediatest"
	_ "knov/internal/test/metadatatest"
	_ "knov/internal/test/reservedtest"
	_ "knov/internal/test/searchtest"
	_ "knov/internal/test/settingstest"
	"knov/internal/thememanager"
	"knov/internal/translation"
)

//go:embed static/*
var staticFS embed.FS

//go:embed themes/builtin
var builtinThemeFS embed.FS

//go:embed docs README.md
var docsFS embed.FS

// @title Knov API
// @version 1.0
// @description KNOV API \n GLHF
// @BasePath /
func main() {
	server.SetStaticFiles(staticFS)
	server.SetDocsFiles(docsFS)
	thememanager.SetBuiltinFiles(builtinThemeFS)
	test.SetDocsFiles(docsFS)

	flags := cli.Parse()
	startTests := flags.StartTests
	testSuite := flags.Suite
	removeTestDir := flags.Remove
	if startTests {
		// before InitAppConfig, which opens app.log against whatever this resolves to
		logging.SetIsolatedLogsDir()
	}

	configmanager.InitAppConfig()
	if err := configmanager.ValidateEnvDefs(); err != nil {
		logging.LogError(logging.KeyApp, "invalid env config: %v", err)
		os.Exit(1)
	}
	translation.Init()

	if startTests {
		if err := test.PrepareIsolatedStorage(); err != nil {
			logging.LogError(logging.KeyApp, "failed to prepare isolated test storage: %v", err)
			os.Exit(1)
		}
	}

	// docs dir doesn't exist yet - nothing has ever been stored, so seed starter content below.
	// Checked after PrepareIsolatedStorage so a --start-tests run stats the isolated data, not
	// the live one.
	_, statErr := os.Stat(pathutils.DocsRoot())
	firstStart := os.IsNotExist(statErr)

	if data, err := staticFS.ReadFile("static/font-awesome/ttf/7-3-1/Font Awesome 7 Free-Solid-900.ttf"); err == nil {
		pdfexport.SetIconFont(data)
	} else {
		logging.LogError(logging.KeyPdfExport, "failed to load pdf export icon font: %v", err)
	}
	loadFonts()

	if err := git.EnsureRemote(); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to configure git remote: %v", err)
	}
	git.EnsureRepoConfig()

	// initialize content storage (creates data/docs and data/media directories)
	if err := contentStorage.Init(); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize content storage: %v", err)
		os.Exit(1)
	}

	// initialize content handlers
	contentHandler.Init()

	// initialize parsers
	parser.Init()

	// initialize storage backends
	appConfig := configmanager.GetAppConfig()

	if err := configStorage.Init(appConfig.ConfigStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize config storage: %v", err)
		os.Exit(1)
	}

	if err := metadataStorage.Init(appConfig.MetadataStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize metadata storage: %v", err)
		os.Exit(1)
	}

	if err := kanbanStorage.Init(appConfig.KanbanEventsEnabled, appConfig.KanbanEventsProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize kanban storage: %v", err)
		os.Exit(1)
	}

	if err := cacheStorage.Init(appConfig.CacheStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize cache storage: %v", err)
		os.Exit(1)
	}

	if err := searchStorage.Init(appConfig.SearchStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize search storage: %v", err)
		os.Exit(1)
	}

	if err := trackerStorage.Init(appConfig.TrackerEnabled, appConfig.TrackerStorageProvider, appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize tracker storage: %v", err)
		os.Exit(1)
	}

	if err := chatStorage.Init(appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize chat storage: %v", err)
		os.Exit(1)
	}

	if err := notificationStorage.Init(appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize notification storage: %v", err)
		os.Exit(1)
	}

	if err := jobStorage.Init(appConfig.StoragePath); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize job storage: %v", err)
		os.Exit(1)
	}

	if err := configmanager.InitSettings(); err != nil {
		logging.LogError(logging.KeyApp, "failed to initialize settings: %v", err)
		os.Exit(1)
	}
	configmanager.LoadThemeSettings()
	translation.SetLanguage(configmanager.GetLanguage())

	thememanager.InitThemeManager()
	// register filter index regeneration to run after every metadata rebuild
	files.OnMetadataRebuild = filter.RegenerateAllIndexes
	// keep kanban board order and dashboard widgets from going stale on a file rename/move
	files.OnFileMoved = func(oldRel, newRel pathutils.DocsRel) {
		kanban.PatchPathForMove(oldRel, newRel)
		dashboard.PatchFilePathForMove(oldRel, newRel)
	}

	// knov --start-tests runs headless against the isolated storage set up above, then exits -
	// never starts the scheduler or HTTP server, and never touches live storage.
	if startTests {
		runHeadlessTests(testSuite, removeTestDir)
	}

	// after config/theme/OnMetadataRebuild are wired up, since a resumed job's background
	// cache rebuild depends on them
	job.RecoverInterrupted()

	if firstStart {
		if err := job.RunTestdataSetup(); err != nil {
			logging.LogError(logging.KeyApp, "failed to seed starter docs: %v", err)
		}
	}

	go func() {
		if err := job.RunSearchReindex(); err != nil {
			logging.LogError(logging.KeyApp, "failed to run startup search index: %v", err)
		}
	}()
	go func() {
		time.Sleep(2 * time.Minute)
		if err := job.RunMetadataRebuild(); err != nil {
			logging.LogError(logging.KeyApp, "failed to run startup metadata rebuild: %v", err)
		}
	}()

	go func() {
		time.Sleep(5 * time.Minute)
		job.Start()
	}()

	server.StartServerChi()
}

// runHeadlessTests runs the named test suite, or every registered suite if name is empty (via
// test.RunAllTestsAndLog), against the isolated storage already prepared by
// test.PrepareIsolatedStorage, prints a summary, and exits the process - `knov --start-tests`
// never starts the scheduler or HTTP server. If removeTestDir is set (knov --start-tests
// --remove), test.TempRoot is deleted before exiting either way.
func runHeadlessTests(name string, removeTestDir bool) {
	result, err := test.RunAllTestsAndLog(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "test run failed: %v\n", err)
		exitHeadlessTests(removeTestDir, 1)
	}

	for _, c := range result.Cases {
		switch {
		case c.Skipped:
			fmt.Printf("SKIP %s: %s\n", c.Name, c.Actual)
		case !c.Success:
			fmt.Printf("FAIL %s: %s\n", c.Name, c.Error)
		}
	}
	fmt.Printf("%d passed, %d skipped, %d failed, %d total\n", result.Passed, result.Skipped, result.Failed, result.Total)

	code := 0
	if !result.Success {
		code = 1
	}
	exitHeadlessTests(removeTestDir, code)
}

// exitHeadlessTests optionally removes the knov_temp_test scratch directory, then exits with
// code - the single exit point for runHeadlessTests so --remove is honored on every path.
func exitHeadlessTests(removeTestDir bool, code int) {
	if removeTestDir {
		files.WaitForCacheRefreshes()
		closeStorages()
		logging.CloseFiles()
		if err := test.RemoveIsolatedStorage(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to remove knov_temp_test: %v\n", err)
		}
	}
	os.Exit(code)
}

// loadFonts registers every embedded font family from the fonts manifest
// with pdfexport (families with no Dir are skipped — those are core fonts
// needing no registration). Families that ship no bold/italic/boldItalic
// pass nil for those styles, and pdfexport falls back to its nearest
// available style at render time.
func loadFonts() {
	read := func(dir, name string) []byte {
		if name == "" {
			return nil
		}
		data, err := staticFS.ReadFile("static/fonts/" + dir + "/" + name)
		if err != nil {
			logging.LogError(logging.KeyPdfExport, "failed to load pdf export font %s/%s: %v", dir, name, err)
			return nil
		}
		return data
	}
	for _, f := range fonts.Families {
		if f.Dir == "" {
			continue
		}
		pdfexport.RegisterFont(f.Name, read(f.Dir, f.Regular), read(f.Dir, f.Bold), read(f.Dir, f.Italic), read(f.Dir, f.BoldItalic))
	}
}

// closeStorages closes the database of every sqlite-backed storage, so the scratch directory of a
// test run can be removed on a host that can not delete open files (windows).
func closeStorages() {
	cacheStorage.Close()
	chatStorage.Close()
	jobStorage.Close()
	kanbanStorage.Close()
	metadataStorage.Close()
	notificationStorage.Close()
	searchStorage.Close()
	trackerStorage.Close()
}
