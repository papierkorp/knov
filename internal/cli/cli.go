// Package cli parses knov's command-line flags.
package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"knov/internal/test"
	"knov/internal/version"
)

// Flags holds the parsed command-line flags for a knov invocation.
type Flags struct {
	StartTests bool
	Suite      string
	Remove     bool
}

// Parse parses os.Args[1:]. --help prints usage and exits 0, --version prints
// version info and exits 0, and any unrecognized or stray argument prints usage
// and exits 2 - knov never starts with wrong/unknown parameters. --start-tests
// optionally takes a trailing suite name (e.g. "--start-tests filter") to run
// only that suite; with no name, every suite runs.
func Parse() Flags {
	fs := flag.NewFlagSet("knov", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: knov [flags] [suite]\n\nflags:\n")
		fs.VisitAll(func(fl *flag.Flag) {
			fmt.Fprintf(fs.Output(), "  --%s\n    \t%s\n", fl.Name, fl.Usage)
		})
		fmt.Fprintf(fs.Output(), "\navailable suites for --start-tests: %s\n", strings.Join(test.SuiteNames(), ", "))
		fmt.Fprintf(fs.Output(), "\nnote: [suite] must be the last argument, after every flag\n"+
			"(e.g. \"--start-tests filter --remove\" fails, \"--start-tests --remove filter\" works) -\n"+
			"flag parsing stops at the first non-flag argument, so anything after [suite] is\n"+
			"treated as a stray argument instead of a flag.\n")
	}

	var f Flags
	var showVersion bool
	fs.BoolVar(&f.StartTests, "start-tests", false, "run the in-app test suite headlessly and exit, optionally followed by a suite name to run only that suite")
	fs.BoolVar(&f.Remove, "remove", false, "with --start-tests, remove the isolated test storage after the run")
	fs.BoolVar(&showVersion, "version", false, "print version information and exit")

	_ = fs.Parse(os.Args[1:])

	// flag.Parse only rejects unknown -flags; a bare stray word is left in fs.Args() instead of
	// erroring. The only stray word allowed is a single suite name following --start-tests.
	switch {
	case fs.NArg() == 0:
	case fs.NArg() == 1 && f.StartTests:
		f.Suite = fs.Arg(0)
	default:
		fs.Usage()
		os.Exit(2)
	}

	if showVersion {
		fmt.Printf("knov %s (build %s, built %s)\n%s\n", version.Version, version.Build, version.BuildTime, version.LastCommitMessage)
		os.Exit(0)
	}

	return f
}
