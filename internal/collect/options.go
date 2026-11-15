// Package collect implements the `collect` subcommand: option parsing, the
// run and the writers of its outputs.
package collect

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
// `doubleagent collect` and without its worker-shell paragraph.
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
		"--full":          &raw.options.Full,
		"--no-secrets":    &raw.options.NoSecrets,
		"--no-projects":   &raw.options.NoProjects,
		"--no-docker":     &raw.options.NoDocker,
		"--env-scan-home": &raw.options.EnvScanHome,
		"-k":              &raw.options.KeepStaging,
		"--keep-staging":  &raw.options.KeepStaging,
		"--inventory":     &raw.options.Inventory,
		"-q":              &raw.options.Quiet,
		"--quiet":         &raw.options.Quiet,
		"--list":          &raw.list,
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
// current directory and listed once, as the sh collector does. A path is never
// prefixed with the root: it names a place on the machine running the
// collector. An empty path is dropped; a relative path that does not exist is
// joined to the current directory unchanged.
func ResolveProjects(options Options, getwd func() (string, error)) (Options, error) {
	var resolved []string
	seen := map[string]bool{}
	for _, project := range options.Projects {
		if project == "" {
			continue
		}
		if !filepath.IsAbs(project) {
			cwd, err := getwd()
			if err != nil {
				return options, fmt.Errorf("resolve project %s: %w", project, err)
			}
			project = absoluteProject(cwd, project)
		}
		if !seen[project] {
			seen[project] = true
			resolved = append(resolved, project)
		}
	}
	options.Projects = resolved
	return options, nil
}

// absoluteProject joins the relative PROJECT to CWD. An existing directory is
// cleaned (the shell's `cd` then `pwd`); anything else keeps its spelling,
// minus one leading "./".
func absoluteProject(cwd, project string) string {
	joined := filepath.Join(cwd, project)
	if isDirectory(joined) {
		return joined
	}
	return cwd + "/" + strings.TrimPrefix(project, "./")
}

// isDirectory reports whether PATH is a directory or a link to one. The
// command line names the base of a collection, so following a link here is the
// shell's own `[ -d ]` and `cd`.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
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

// ResolveRoot returns OPTIONS with Root made absolute. A root that is not a
// directory is an error (exit code 2). The root "/" becomes "", which means
// live mode.
func ResolveRoot(options Options, getwd func() (string, error)) (Options, error) {
	if options.Root == "" {
		return options, nil
	}
	root := options.Root
	checked := root
	if !filepath.IsAbs(root) {
		cwd, err := getwd()
		if err != nil {
			return options, fmt.Errorf("resolve root %s: %w", root, err)
		}
		checked = cwd + "/" + root
		root = filepath.Join(cwd, root)
	}
	if !isDirectory(checked) {
		return options, &RootError{Root: options.Root}
	}
	root = filepath.Clean(root)
	if root == "/" {
		root = ""
	}
	options.Root = root
	return options, nil
}
