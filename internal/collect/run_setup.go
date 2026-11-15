package collect

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/hostcmd"
	"github.com/seanthegeek/doubleagent/internal/inventory"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/projects"
	"github.com/seanthegeek/doubleagent/internal/relocation"
	"github.com/seanthegeek/doubleagent/internal/runlog"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/version"
	"github.com/seanthegeek/doubleagent/internal/volumes"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// runState is one run: its options, its output names and open files, and what
// each step found for the steps after it.
type runState struct {
	options Options
	host    environment
	stdout  io.Writer
	stderr  io.Writer
	tables  *catalog.Tables
	plutil  hostcmd.Plutil
	started time.Time

	hostname     string
	name         string
	outDir       string
	stageDir     string
	archivePath  string
	logPath      string
	manifestPath string
	summaryPath  string

	log          *runlog.Log
	manifestFile *os.File
	rows         *bufio.Writer
	manifest     *manifest.Writer
	collector    *walk.Collector

	homes         []users.Home
	projects      []projects.Project
	docker        volumes.Counts
	dockerRecords *inventory.Aggregator
	notes         []string
	relocation    relocation.Result
}

// prepareRun resolves the options and the output names, as the sh collector's
// resolve_extra_projects, resolve_root, check_prerequisites,
// detect_capabilities and set_output_names do. Every failure is a fatalError.
func prepareRun(options Options, host environment, stdout, stderr io.Writer,
) (*runState, error) {
	options, err := ResolveProjects(options, host.getwd)
	if err == nil {
		options, err = ResolveRoot(options, host.getwd)
	}
	if err == nil && !options.Inventory {
		options.OutDir, err = resolveOutDir(options.OutDir, host.getwd)
	}
	tables, tablesErr := catalog.Load()
	if err == nil {
		err = tablesErr
	}
	if err != nil {
		if _, ok := err.(*fatalError); !ok {
			err = &fatalError{message: err.Error()}
		}
		return nil, err
	}
	state := &runState{options: options, host: host, stdout: stdout, stderr: stderr,
		tables: tables, plutil: hostcmd.NewPlutil(host.runner), started: host.now().UTC(),
		dockerRecords: inventory.NewAggregator()}
	state.setOutputNames()
	return state, nil
}

// resolveOutDir creates the output directory when it is missing and returns
// it absolute, as the shell's `cd DIR && pwd` gives it. A directory that
// cannot be created, entered or written is fatal, with the sh collector's
// messages.
func resolveOutDir(outDir string, getwd func() (string, error)) (string, error) {
	if !isDirectory(outDir) {
		if err := os.MkdirAll(outDir, 0o777); err != nil || outDir == "" {
			return "", &fatalError{message: "Cannot create output dir: " + outDir}
		}
	}
	absolute := outDir
	if !filepath.IsAbs(outDir) {
		cwd, err := getwd()
		if err != nil {
			return "", &fatalError{message: "cd: can't cd to " + outDir}
		}
		absolute = filepath.Join(cwd, outDir)
	}
	absolute = filepath.Clean(absolute)
	if !canAccess(absolute, accessEnter) {
		return "", &fatalError{message: "cd: can't cd to " + outDir}
	}
	if !canAccess(absolute, accessWrite) {
		return "", &fatalError{message: "Output dir not writable: " + absolute}
	}
	return absolute, nil
}

// setOutputNames names the run's outputs after the host and the start time,
// as set_output_names does.
func (state *runState) setOutputNames() {
	state.hostname = hostname()
	state.name = safeHostname(state.hostname) + "_" + state.started.Format("20060102T150405Z") +
		"_agent-artifacts"
	base := state.options.OutDir + "/"
	state.outDir = state.options.OutDir
	state.stageDir = base + ".stage-" + state.name
	state.archivePath = base + state.name + ".tar.gz"
	state.logPath = base + state.name + ".log"
	state.manifestPath = base + state.name + ".manifest.jsonl"
	state.summaryPath = base + state.name + ".collection.json"
}

// safeHostname replaces every byte of name that is not an ASCII letter,
// digit, `.`, `_` or `-` with `-`, as `tr -c 'A-Za-z0-9._-' '-'` does.
func safeHostname(name string) string {
	safe := []byte(name)
	for i, char := range safe {
		switch {
		case 'a' <= char && char <= 'z', 'A' <= char && char <= 'Z', '0' <= char && char <= '9',
			char == '.', char == '_', char == '-':
		default:
			safe[i] = '-'
		}
	}
	return string(safe)
}

// openOutputs creates the staging directory, the log and the manifest, as
// create_output_files does, and the collector every walk shares. Under
// --inventory nothing is created: the log goes to the null device and the
// rows nowhere.
func (state *runState) openOutputs() error {
	logPath, rows := os.DevNull, io.Writer(io.Discard)
	physical := ""
	if !state.options.Inventory {
		if err := os.MkdirAll(state.stageDir+"/fs", 0o777); err != nil {
			return &fatalError{message: "Cannot create staging dir"}
		}
		var err error
		if physical, err = stage.PhysicalDir(state.stageDir); err != nil {
			return &fatalError{message: "Cannot create staging dir"}
		}
		if state.manifestFile, err = os.Create(state.manifestPath); err != nil {
			return &fatalError{message: err.Error()}
		}
		state.rows = bufio.NewWriter(state.manifestFile)
		logPath, rows = state.logPath, state.rows
	}
	log, err := runlog.Open(logPath, state.options.Quiet, state.stderr, state.host.now)
	if err != nil {
		return &fatalError{message: err.Error()}
	}
	state.log, state.manifest = log, manifest.NewWriter(rows)
	state.collector = state.newCollector(physical)
	return nil
}

// newCollector returns the walk settings of the run: the active exclusions
// (none under --full), the credential test and the staging directory.
func (state *runState) newCollector(physical string) *walk.Collector {
	excludes := state.tables.Excludes
	if state.options.Full {
		excludes = nil
	}
	root := state.options.Root
	return &walk.Collector{
		Tables: state.tables, Excludes: excludes, Secret: walk.MatchAny(state.tables.SecretGlobs),
		Log: state.log, Manifest: state.manifest, Root: root,
		Stager:  stage.Stager{Dir: state.stageDir, Physical: physical, Root: root},
		MaxSize: state.options.MaxFileBytes, NoSecrets: state.options.NoSecrets,
		Inventory: state.options.Inventory, Records: state.dockerRecords,
	}
}

// mode is the run's mode: image under -r, live otherwise.
func (state *runState) mode() string {
	if state.options.Root != "" {
		return "image"
	}
	return "live"
}

// startLog writes the first log lines, as log_start does: the version, host,
// mode and options, then the warnings. The sizes are printed from their byte
// values in whole MiB.
func (state *runState) startLog(context.Context) error {
	system := uname()
	root := state.options.Root
	if root == "" {
		root = "/"
	}
	options := state.options
	lines := []string{
		fmt.Sprintf("%s %s starting on %s (%s %s) mode=%s root=%s uid=%d", summary.Tool,
			version.String(), state.hostname, system.name, system.release, state.mode(), root,
			os.Getuid()),
		fmt.Sprintf("hash=crypto/sha256 max_file_size=%dMB full=%s no_secrets=%s no_docker=%s"+
			" env_scan_home=%s env_scan_max_size=%dMB", options.MaxFileBytes/mebibyte,
			flag(options.Full), flag(options.NoSecrets), flag(options.NoDocker),
			flag(options.EnvScanHome), options.EnvScanMaxBytes/mebibyte),
	}
	if state.mode() == "live" && os.Getuid() != 0 {
		lines = append(lines, "WARNING: not running as root; other users' homes will probably be unreadable")
	}
	if options.Inventory && options.OutDirSet {
		lines = append(lines, "NOTE: --inventory writes nothing; -o "+options.OutDir+" ignored")
	}
	for _, line := range lines {
		if err := state.log.Line(line); err != nil {
			return err
		}
	}
	return nil
}

// flag returns 1 or 0, as the sh collector prints an option in the log.
func flag(set bool) string {
	if set {
		return "1"
	}
	return "0"
}

// line logs text, or, before the log is open, prints it on standard error
// unless the run is quiet. A failed write has nowhere left to be reported.
func (state *runState) line(text string) {
	if state.log != nil {
		_ = state.log.Line(text)
		return
	}
	if !state.options.Quiet {
		io.WriteString(state.stderr, text+"\n")
	}
}

// cleanup closes the run's files and removes the staging directory unless
// -k was given, as the sh collector's cleanup does on every exit.
func (state *runState) cleanup() {
	if state.rows != nil {
		_ = state.rows.Flush()
	}
	if state.manifestFile != nil {
		_ = state.manifestFile.Close()
	}
	if state.log != nil {
		_ = state.log.Close()
	}
	if !state.options.KeepStaging && !state.options.Inventory {
		_ = os.RemoveAll(state.stageDir)
	}
}
