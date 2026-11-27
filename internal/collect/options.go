package collect

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Options holds every option of `collect`, with their defaults.
type Options struct {
	OutDir          string
	OutDirSet       bool
	Root            string
	UsersFilter     string
	Projects        []string
	Full            bool
	NoSecrets       bool
	NoProjects      bool
	NoDocker        bool
	EnvScanHome     bool
	KeepStaging     bool
	Inventory       bool
	Quiet           bool
	MaxFileBytes    int64
	EnvScanMaxBytes int64
	// NoPreserveAtime, set by --no-preserve-atime, opens every file for
	// reading alone. Without it, a Windows open where a read could move the
	// access time also asks for the right to change attributes, which
	// security software may watch for, so it can ask Windows to keep that
	// time; with it, reading may change the time on disk. A row's atime is
	// taken before the copy or the relocation search's read either way. Unix
	// files are always opened for reading alone, so it changes nothing there.
	NoPreserveAtime bool
	// lexicalRoot is Root made absolute as typed, before its components
	// were respelled from the disk; "" with no root.
	lexicalRoot string
}

// Action says what the command line asked for.
type Action int

// The actions a command line can ask for.
const (
	ActRun Action = iota
	ActList
	ActVersion
	ActHelp
)

const mebibyte = 1024 * 1024

// UsageText is the help text of `doubleagent collect`.
const UsageText = `Usage: doubleagent collect [options]

  -o, --output DIR        Directory for the archive (default: current dir)
  -r, --root DIR          Alternate root, e.g. a mounted disk image (image mode)
  -u, --users LIST        Comma-separated usernames to collect (default: all)
  -p, --project DIR       Extra project directory to collect (repeatable)
      --full              Disable default size exclusions (model blobs, caches,
                          extension binaries)
      --no-secrets        Skip credential files instead of collecting them
      --no-projects       Skip project-level artifact discovery
      --no-docker         Skip Docker and Podman volume enumeration
      --max-file-size MB  Skip files larger than this (default 256, 0 = none)
      --env-scan-home     Also search every other file of each home and project
                          for the relocation variables (slow)
      --env-scan-max-size MB
                          Skip files larger than this in that search (default
                          1, 0 = none)
      --no-preserve-atime Open files read-only; reading may change their
                          access time on disk
  -k, --keep-staging      Keep the staging directory after archiving
      --inventory         Write nothing; print one JSON line per user and agent
                          found (file count, bytes, first and last mtime) to
                          stdout. -o is ignored
      --list              Print the artifact catalog and exit
  -q, --quiet             Only print the final summary
  -V, --version           Print version and exit
  -h, --help              This help

Run as root to collect every user's home.
`

// rawOptions holds the option values as typed, before the sizes are checked,
// and the switches that print instead of collecting.
type rawOptions struct {
	options Options
	maxMB   string
	scanMB  string
	list    bool
	version bool
	help    bool
}

// usageError returns the usage error the caller prints for a mistake flag
// or the argument count finds: "doubleagent collect: " and message, then
// UsageText.
func usageError(message string) error {
	return errors.New("doubleagent collect: " + message + "\n" + strings.TrimSuffix(UsageText, "\n"))
}

// flagSet returns the flag set of `collect`, reading into raw. An option with
// a short and a long name is one variable under two names. Parse prints
// nothing: ParseOptions returns the mistake for the caller to print.
func (raw *rawOptions) flagSet() *flag.FlagSet {
	set := flag.NewFlagSet("collect", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	options := &raw.options
	output := func(value string) error {
		options.OutDir, options.OutDirSet = value, true
		return nil
	}
	project := func(value string) error {
		options.Projects = append(options.Projects, value)
		return nil
	}
	set.Func("o", "", output)
	set.Func("output", "", output)
	set.StringVar(&options.Root, "r", "", "")
	set.StringVar(&options.Root, "root", "", "")
	set.StringVar(&options.UsersFilter, "u", "", "")
	set.StringVar(&options.UsersFilter, "users", "", "")
	set.Func("p", "", project)
	set.Func("project", "", project)
	set.BoolVar(&options.Full, "full", false, "")
	set.BoolVar(&options.NoSecrets, "no-secrets", false, "")
	set.BoolVar(&options.NoProjects, "no-projects", false, "")
	set.BoolVar(&options.NoDocker, "no-docker", false, "")
	set.StringVar(&raw.maxMB, "max-file-size", "256", "")
	set.BoolVar(&options.EnvScanHome, "env-scan-home", false, "")
	set.StringVar(&raw.scanMB, "env-scan-max-size", "1", "")
	set.BoolVar(&options.NoPreserveAtime, "no-preserve-atime", false, "")
	set.BoolVar(&options.KeepStaging, "k", false, "")
	set.BoolVar(&options.KeepStaging, "keep-staging", false, "")
	set.BoolVar(&options.Inventory, "inventory", false, "")
	set.BoolVar(&raw.list, "list", false, "")
	set.BoolVar(&options.Quiet, "q", false, "")
	set.BoolVar(&options.Quiet, "quiet", false, "")
	set.BoolVar(&raw.version, "V", false, "")
	set.BoolVar(&raw.version, "version", false, "")
	set.BoolVar(&raw.help, "h", false, "")
	set.BoolVar(&raw.help, "help", false, "")
	return set
}

// mebibytes converts TEXT, a non-empty run of ASCII digits, to bytes. Text
// that is empty, has another character (a sign included) or overflows is
// not a size.
func mebibytes(text string) (int64, bool) {
	count, err := strconv.ParseUint(text, 10, 64)
	if err != nil || count > (1<<62)/mebibyte {
		return 0, false
	}
	return int64(count) * mebibyte, true
}

// Error returns the message naming the root.
func (e *RootError) Error() string {
	return "Root is not a directory: " + e.Root
}

// ParseOptions reads the arguments after `collect` with the flag package, so
// each option is also accepted with one dash or two and with its value after
// `=`. What flag cannot state is checked once the line is read.
func ParseOptions(args []string) (Options, Action, error) {
	raw := rawOptions{options: Options{OutDir: "."}}
	set := raw.flagSet()
	if err := set.Parse(args); err != nil {
		return Options{}, ActRun, usageError(err.Error())
	}
	// collect takes no argument that is not an option.
	if set.NArg() > 0 {
		return Options{}, ActRun, usageError("unexpected argument: " + set.Arg(0))
	}
	// A size is whole MiB; its error message is printed without the usage.
	maxBytes, ok := mebibytes(raw.maxMB)
	if !ok {
		return Options{}, ActRun, errors.New("Invalid --max-file-size: " + raw.maxMB)
	}
	scanBytes, ok := mebibytes(raw.scanMB)
	if !ok {
		return Options{}, ActRun, errors.New("Invalid --env-scan-max-size: " + raw.scanMB)
	}
	options := raw.options
	options.MaxFileBytes, options.EnvScanMaxBytes = maxBytes, scanBytes
	// On a line without a mistake, --help, then --version, then --list
	// replace the collection.
	switch {
	case raw.help:
		return options, ActHelp, nil
	case raw.version:
		return options, ActVersion, nil
	case raw.list:
		return options, ActList, nil
	}
	return options, ActRun, nil
}

// ResolveProjects returns OPTIONS with every -p path made absolute against
// the current directory (absoluteProject, per platform) and listed once: on
// a live run two spellings the file system calls one entry are one; on an
// image run only equal bytes, the rest judged under the root check
// (reachedProjects). A path is never prefixed with the root: it names a
// place on the machine the run is on. An empty path is dropped; the current
// directory is asked for only when a path is relative.
func ResolveProjects(options Options, getwd func() (string, error)) (Options, error) {
	var resolved []string
	// The root is not resolved yet, so an image run asks the file system
	// nothing about the analyst's paths here; a live run asks about its own.
	var same syspath.SameEntry
	if options.Root == "" {
		same = follow.SameEntry(nil)
	}
	seen := syspath.NewEntries[struct{}](same)
	for _, project := range options.Projects {
		if project == "" {
			continue
		}
		cwd := ""
		if !filepath.IsAbs(project) {
			var err error
			if cwd, err = getwd(); err != nil {
				return options, fmt.Errorf("resolve project %s: %w", project, err)
			}
		}
		project = absoluteProject(cwd, project)
		if seen.Add(project, struct{}{}) {
			resolved = append(resolved, project)
		}
	}
	options.Projects = resolved
	return options, nil
}

// respellProjects returns the -p paths, `.` and `..` removed by name on
// Windows, with existing components spelled as the disk holds them and each
// path listed once by its bytes: below root in image mode, so a path outside
// the image stays as typed and is never looked at, and from the drive on a
// live run. A path starting with lexicalRoot, the root as typed (which may
// hold a short name or another case), first has that start replaced by root,
// so a -p spelled like -r stays under it; another spelling of the root
// counts as outside the image. It runs after ResolveProjects and the root,
// so a -p error is reported before a -r one.
func respellProjects(projects []string, lexicalRoot, root string) []string {
	var respelled []string
	seen := map[string]bool{}
	lexicalRoot = syspath.Opened(lexicalRoot)
	for _, project := range projects {
		// On Windows, `.` and `..` are removed by name as Win32 removes
		// them, so one string is checked, opened, staged and logged.
		project = syspath.Opened(project)
		if rest, found := syspath.CutPrefix(project, lexicalRoot+"/"); root != "" && found {
			project = root + "/" + rest
		} else if root != "" && syspath.Equal(project, lexicalRoot) {
			project = root
		}
		project = fsmeta.OnDiskPath(root, project)
		if !seen[project] {
			seen[project] = true
			respelled = append(respelled, project)
		}
	}
	return respelled
}

// RootError reports a root that is not a directory, exit code 2.
type RootError struct {
	Root string
}

// isDirectory reports whether PATH is a directory or a link to one. The
// command line names the base of a collection, so a link the operator typed
// is followed.
func isDirectory(path string) bool {
	info, err := osfile.Stat(path)
	return err == nil && info.IsDir()
}

// ResolveRoot returns OPTIONS with Root made absolute and canonical
// (absoluteRoot, per platform); the current directory is asked for only when
// the root is relative. A root that is not a directory is an error (exit
// code 2). A root that liveRoot says names the live system ("/" on Unix,
// none on Windows) becomes "", live mode.
func ResolveRoot(options Options, getwd func() (string, error)) (Options, error) {
	if options.Root == "" {
		return options, nil
	}
	cwd := ""
	if !filepath.IsAbs(options.Root) {
		var err error
		if cwd, err = getwd(); err != nil {
			return options, fmt.Errorf("resolve root %s: %w", options.Root, err)
		}
	}
	root, checked := absoluteRoot(cwd, options.Root)
	if !isDirectory(checked) {
		return options, &RootError{Root: options.Root}
	}
	lexical := typedRoot(cwd, options.Root, root)
	if liveRoot(root) {
		root, lexical = "", ""
	}
	options.Root, options.lexicalRoot = root, lexical
	return options, nil
}
