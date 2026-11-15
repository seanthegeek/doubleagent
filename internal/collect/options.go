// Package collect implements the `collect` subcommand: option parsing, the
// run and the writers of its outputs.
package collect

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

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

// UsageError is a command-line mistake, exit code 1. Message is printed first
// when it is not empty; the usage text follows when ShowUsage is set.
type UsageError struct {
	Message   string
	ShowUsage bool
}

// Error returns the message, or a fixed text when only the usage is shown.
func (e *UsageError) Error() string {
	if e.Message == "" {
		return "missing option value"
	}
	return e.Message
}

// Print writes the message and, when asked, the usage text to W.
func (e *UsageError) Print(w io.Writer) {
	if e.Message != "" {
		fmt.Fprintln(w, e.Message)
	}
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

// rawOptions holds the option values as typed, before the sizes are checked.
type rawOptions struct {
	options Options
	maxMB   string
	scanMB  string
	list    bool
}

// ParseOptions reads the arguments after `collect`. As in the sh collector,
// --version and --help act at their position in the line, so an earlier
// mistake still fails first, and a bad size is reported only after --list has
// been seen.
func ParseOptions(args []string) (Options, Action, error) {
	raw := rawOptions{
		options: Options{OutDir: "."},
		maxMB:   "256",
		scanMB:  "1",
	}
	for i := 0; i < len(args); i++ {
		consumed, action, err := raw.apply(args[i:])
		if err != nil {
			return Options{}, ActRun, err
		}
		if action != ActRun {
			return raw.options, action, nil
		}
		i += consumed
	}
	if raw.list {
		return raw.options, ActList, nil
	}
	if err := raw.setSizes(); err != nil {
		return Options{}, ActRun, err
	}
	return raw.options, ActRun, nil
}

// apply handles the option at ARGS[0]. It returns how many value arguments it
// consumed, and an action other than ActRun when the line ends there.
func (raw *rawOptions) apply(args []string) (int, Action, error) {
	switch args[0] {
	case "-V", "--version":
		return 0, ActVersion, nil
	case "-h", "--help":
		return 0, ActHelp, nil
	case "-o", "--output", "-r", "--root", "-u", "--users", "-p", "--project",
		"--max-file-size", "--env-scan-max-size":
		if len(args) < 2 {
			return 0, ActRun, &UsageError{ShowUsage: true}
		}
		raw.setValue(args[0], args[1])
		return 1, ActRun, nil
	}
	if !raw.setFlag(args[0]) {
		return 0, ActRun, &UsageError{Message: "Unknown option: " + args[0], ShowUsage: true}
	}
	return 0, ActRun, nil
}

// setValue stores VALUE for the option NAME that takes one.
func (raw *rawOptions) setValue(name, value string) {
	switch name {
	case "-o", "--output":
		raw.options.OutDir, raw.options.OutDirSet = value, true
	case "-r", "--root":
		raw.options.Root = value
	case "-u", "--users":
		raw.options.UsersFilter = value
	case "-p", "--project":
		raw.options.Projects = append(raw.options.Projects, value)
	case "--max-file-size":
		raw.maxMB = value
	case "--env-scan-max-size":
		raw.scanMB = value
	}
}

// setFlag sets the option NAME that takes no value and reports whether NAME is
// one.
func (raw *rawOptions) setFlag(name string) bool {
	flags := map[string]*bool{
		"--full":              &raw.options.Full,
		"--no-secrets":        &raw.options.NoSecrets,
		"--no-projects":       &raw.options.NoProjects,
		"--no-docker":         &raw.options.NoDocker,
		"--env-scan-home":     &raw.options.EnvScanHome,
		"--no-preserve-atime": &raw.options.NoPreserveAtime,
		"-k":                  &raw.options.KeepStaging,
		"--keep-staging":      &raw.options.KeepStaging,
		"--inventory":         &raw.options.Inventory,
		"-q":                  &raw.options.Quiet,
		"--quiet":             &raw.options.Quiet,
		"--list":              &raw.list,
	}
	flag, ok := flags[name]
	if ok {
		*flag = true
	}
	return ok
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
// current directory (absoluteProject, per platform) and listed once, paths
// that differ only in case being one on Windows. A path is never prefixed
// with the root: it names a place on the machine running the collector. An
// empty path is dropped; the current directory is asked for only when a path
// or, on Unix with a relative -p, the root is relative (imageCheck).
func ResolveProjects(options Options, getwd func() (string, error)) (Options, error) {
	var resolved []string
	seen := map[string]bool{}
	check := imageCheck(options, getwd)
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
		if key := syspath.Fold(project); !seen[key] {
			seen[key] = true
			resolved = append(resolved, project)
		}
	}
	options.Projects = resolved
	return options, nil
}

// respellProjects returns the -p paths with their existing components
// spelled as the disk holds them, each listed once: below root in image
// mode, so a path outside the image stays as typed and is never looked at,
// and from the drive on a live run. A path starting with lexicalRoot, the
// root as typed (which may hold a short name or another case the respelled
// root lost), first has that start replaced by root, so a -p spelled like
// -r stays under it; a -p reaching the root through another short or long
// spelling or another case of a non-ASCII letter counts as outside the
// image. It runs once the root is resolved, after ResolveProjects, so the
// order of the two errors stays the sh collector's.
func respellProjects(projects []string, lexicalRoot, root string) []string {
	var respelled []string
	seen := map[string]bool{}
	for _, project := range projects {
		if rest, found := syspath.CutPrefix(project, lexicalRoot+"/"); root != "" && found {
			project = root + "/" + rest
		} else if root != "" && syspath.Equal(project, lexicalRoot) {
			project = root
		}
		project = fsmeta.OnDiskPath(root, project)
		if key := syspath.Fold(project); !seen[key] {
			seen[key] = true
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
