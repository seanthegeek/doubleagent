// Package collect implements the `collect` subcommand: option parsing, the
// run and the writers of its outputs.
package collect

import (
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

// Options holds every option of `collect`, with the defaults the sh collector
// uses.
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
	// NoPreserveAtime, set by --no-preserve-atime, opens every file the run
	// reads for reading alone. Without it, Windows opens ask for the right
	// to change the file's attributes, which security software may watch
	// for, wherever a read could move the access time, so they can ask
	// Windows to leave that time as it was; with it, reading may change the
	// time on disk. A manifest row's atime is taken before the copy, and for
	// a file the relocation search reads, before that read. On Linux, macOS
	// and other Unix systems files are always opened for reading alone, so
	// it changes nothing there.
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

// UsageError is a command-line mistake, exit code 1. Message is printed
// first; the usage text follows when ShowUsage is set.
type UsageError struct {
	Message   string
	ShowUsage bool
}

// Error returns the message.
func (e *UsageError) Error() string {
	return e.Message
}

// Print writes the message and, when asked, the usage text to W.
func (e *UsageError) Print(w io.Writer) {
	fmt.Fprintln(w, e.Message)
	if e.ShowUsage {
		fmt.Fprint(w, UsageText)
	}
}

// UsageText is the help text: the sh collector's, with the program name
// `doubleagent collect`, without its worker-shell paragraph and with the
// binary's own --no-preserve-atime.
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
// and the switches that end the line.
type rawOptions struct {
	options Options
	maxMB   string
	scanMB  string
	list    bool
	version bool
	help    bool
}

// ParseOptions reads the arguments after `collect` with the flag package, so
// each option is also accepted with one dash or two and with its value after
// `=`. What flag cannot state is checked once the line is read: `collect`
// takes no argument that is not an option; --help, then --version, then
// --list end the line whatever their place; and the two sizes are checked
// last, the max size first, so a bad size is reported only when the line
// runs a collection.
func ParseOptions(args []string) (Options, Action, error) {
	raw := rawOptions{options: Options{OutDir: "."}, maxMB: "256", scanMB: "1"}
	set := raw.flagSet()
	if err := set.Parse(args); err != nil {
		return Options{}, ActRun, &UsageError{Message: "doubleagent collect: " + err.Error(), ShowUsage: true}
	}
	if set.NArg() > 0 {
		return Options{}, ActRun, &UsageError{
			Message: "doubleagent collect: unexpected argument: " + set.Arg(0), ShowUsage: true}
	}
	switch {
	case raw.help:
		return raw.options, ActHelp, nil
	case raw.version:
		return raw.options, ActVersion, nil
	case raw.list:
		return raw.options, ActList, nil
	}
	if err := raw.setSizes(); err != nil {
		return Options{}, ActRun, err
	}
	return raw.options, ActRun, nil
}

// flagSet returns the flag set of `collect`, reading into raw. An option with
// a short and a long name is one variable under two names. Parse prints
// nothing: ParseOptions words the error and the caller prints it with
// UsageText, the documented help, which flag's own listing is not.
func (raw *rawOptions) flagSet() *flag.FlagSet {
	set := flag.NewFlagSet("collect", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	set.Usage = func() {}
	options := &raw.options
	output := func(value string) error {
		options.OutDir, options.OutDirSet = value, true
		return nil
	}
	project := func(value string) error {
		options.Projects = append(options.Projects, value)
		return nil
	}
	for _, name := range []string{"o", "output"} {
		set.Func(name, "", output)
	}
	for _, name := range []string{"p", "project"} {
		set.Func(name, "", project)
	}
	stringOption(set, &options.Root, "r", "root")
	stringOption(set, &options.UsersFilter, "u", "users")
	stringOption(set, &raw.maxMB, "max-file-size")
	stringOption(set, &raw.scanMB, "env-scan-max-size")
	switches := map[*bool][]string{&options.Full: {"full"}, &options.NoSecrets: {"no-secrets"},
		&options.NoProjects: {"no-projects"}, &options.NoDocker: {"no-docker"},
		&options.EnvScanHome: {"env-scan-home"}, &options.NoPreserveAtime: {"no-preserve-atime"},
		&options.KeepStaging: {"k", "keep-staging"}, &options.Inventory: {"inventory"},
		&options.Quiet: {"q", "quiet"}, &raw.list: {"list"}, &raw.version: {"V", "version"},
		&raw.help: {"h", "help"}}
	for target, names := range switches {
		switchOption(set, target, names...)
	}
	return set
}

// stringOption registers target under each of names, keeping its value as
// the default.
func stringOption(set *flag.FlagSet, target *string, names ...string) {
	for _, name := range names {
		set.StringVar(target, name, *target, "")
	}
}

// switchOption registers the boolean target, false by default, under each
// of names.
func switchOption(set *flag.FlagSet, target *bool, names ...string) {
	for _, name := range names {
		set.BoolVar(target, name, false, "")
	}
}

// setSizes converts the two size options from whole MiB to bytes.
func (raw *rawOptions) setSizes() error {
	maxBytes, ok := mebibytes(raw.maxMB)
	if !ok {
		return &UsageError{Message: "Invalid --max-file-size: " + raw.maxMB}
	}
	scanBytes, ok := mebibytes(raw.scanMB)
	if !ok {
		return &UsageError{Message: "Invalid --env-scan-max-size: " + raw.scanMB}
	}
	raw.options.MaxFileBytes, raw.options.EnvScanMaxBytes = maxBytes, scanBytes
	return nil
}

// mebibytes converts TEXT, a non-empty run of ASCII digits, to bytes. Text
// that is empty, has another character or overflows is not a size.
func mebibytes(text string) (int64, bool) {
	if text == "" || strings.Trim(text, "0123456789") != "" {
		return 0, false
	}
	count, err := strconv.ParseInt(text, 10, 64)
	if err != nil || count > (1<<62)/mebibyte {
		return 0, false
	}
	return count * mebibyte, true
}

// ResolveProjects returns OPTIONS with every -p path made absolute against the
// current directory (absoluteProject, per platform) and listed once
// (syspath.Entries): on a live run two spellings the file system calls one
// entry are one; on an image run only equal bytes, the rest judged once the
// root check exists (reachedProjects). A path is never prefixed
// with the root: it names a place on the machine running the collector. An
// empty path is dropped; the current directory is asked for only when a path
// or, on Unix with a relative -p, the root is relative (imageCheck).
func ResolveProjects(options Options, getwd func() (string, error)) (Options, error) {
	var resolved []string
	check := imageCheck(options, getwd)
	// The root is not resolved yet, so an image run asks the file system
	// nothing about the analyst's paths here (reachedProjects does, under
	// the root check); a live run asks about its own.
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
		project = absoluteProject(cwd, project, check)
		if seen.Add(project, struct{}{}) {
			resolved = append(resolved, project)
		}
	}
	options.Projects = resolved
	return options, nil
}

// respellProjects returns the -p paths, `.` and `..` removed by name on
// Windows (syspath.Opened), with existing components spelled as the disk
// holds them, each listed once by its bytes (two spellings of one existing
// directory are one spelling by then): below root in image mode, so a path outside
// the image stays as typed and is never looked at, and from the drive on a
// live run. A path starting with lexicalRoot, the root as typed (which may
// hold a short name or another case the respelled root lost), first has that
// start replaced by root, so a -p spelled like -r stays under it; another
// spelling of the root counts as outside the image. It runs once the root is
// resolved, after ResolveProjects, so the order of the errors is the sh one.
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

// isDirectory reports whether PATH is a directory or a link to one. The
// command line names the base of a collection, so following a link here is the
// shell's own `[ -d ]` and `cd`.
func isDirectory(path string) bool {
	info, err := osfile.Stat(path)
	return err == nil && info.IsDir()
}

// RootError reports a root that is not a directory, exit code 2.
type RootError struct {
	Root string
}

// Error returns the sh collector's text.
func (e *RootError) Error() string {
	return "Root is not a directory: " + e.Root
}

// ResolveRoot returns OPTIONS with Root made absolute and canonical
// (absoluteRoot, per platform); the current directory is asked for only when
// the root is relative. A root that is not a directory is an error
// (exit code 2). A root liveRoot names the live system becomes "", which
// means live mode: "/" on Unix, none on Windows.
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
