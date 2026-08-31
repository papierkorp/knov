// Package cli parses knov's command-line flags.
package cli

import (
	"flag"
	"fmt"
	"os"

	"knov/internal/version"
)

// Flags holds the parsed command-line flags for a knov invocation.
type Flags struct {
	StartTests bool
	Remove     bool
}

// Parse parses os.Args[1:]. --help prints usage and exits 0, --version prints
// version info and exits 0, and any unrecognized or stray argument prints usage
// and exits 2 - knov never starts with wrong/unknown parameters.
func Parse() Flags {
	fs := flag.NewFlagSet("knov", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: knov [flags]\n\nflags:\n")
		fs.VisitAll(func(fl *flag.Flag) {
			fmt.Fprintf(fs.Output(), "  --%s\n    \t%s\n", fl.Name, fl.Usage)
		})
	}

	var f Flags
	var showVersion bool
	fs.BoolVar(&f.StartTests, "start-tests", false, "run the in-app test suite headlessly and exit")
	fs.BoolVar(&f.Remove, "remove", false, "with --start-tests, remove the isolated test storage after the run")
	fs.BoolVar(&showVersion, "version", false, "print version information and exit")

	_ = fs.Parse(os.Args[1:])

	// flag.Parse only rejects unknown -flags; a bare stray word (e.g. "version"
	// without dashes) is left in fs.Args() instead of erroring, so check it here.
	if fs.NArg() > 0 {
		fs.Usage()
		os.Exit(2)
	}

	if showVersion {
		fmt.Printf("knov %s (built %s)\n%s\n", version.Version, version.BuildTime, version.LastCommitMessage)
		os.Exit(0)
	}

	return f
}
