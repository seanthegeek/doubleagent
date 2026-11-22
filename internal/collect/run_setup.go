package collect

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/hostcmd"
	"github.com/seanthegeek/doubleagent/internal/inventory"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/projects"
	"github.com/seanthegeek/doubleagent/internal/relocation"
	"github.com/seanthegeek/doubleagent/internal/runlog"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/syspath"
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
	partialPath  string
	logPath      string
	manifestPath string
	summaryPath  string
	// stageFoldsCase is set where the staging directory takes two names
	// differing only in case for one (keepCaseApart).
	stageFoldsCase bool
	// moving is set once the archive's move to its name has begun, after
	// which a stop request changes nothing.
	moving bool

	log          *runlog.Log
	manifestFile *os.File
	rows         *bufio.Writer
	manifest     *manifest.Writer
	collector    *walk.Collector
	// roots is the image root's check, resolved once; zero on a live run.
	roots walk.RootCheck

	accessUpdates accessUpdates
	opener        *fsmeta.Opener
	notHeld       map[string]bool
	notHeldLines  map[[2]string]bool

	homes         []users.Home
	skippedHomes  []users.Skipped
	profiles      []relocation.Profile
	projects      []projects.Project
	docker        volumes.Counts
	dockerRecords *inventory.Aggregator
	notes         []string
	relocation    relocation.Result
}

// prepareRun resolves the -p paths, the root, the prerequisites, the
// platform's capabilities and the output names. Every failure is a
// fatalError.
func prepareRun(options Options, host environment, stdout, stderr io.Writer,
) (*runState, error) {
	options, err := resolvePaths(options, host.getwd)
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
		dockerRecords: inventory.NewAggregator(), accessUpdates: host.lastAccess()}
	state.roots = walk.NewRootCheck(options.Root)
	state.opener = fsmeta.NewOpener(options.NoPreserveAtime, state.accessUpdates == accessOff)
	state.opener.Report = state.reportNotHeld
	state.notHeld, state.notHeldLines = map[string]bool{}, map[[2]string]bool{}
	state.setOutputNames()
	return state, nil
}

// resolveOutDir creates the output directory when it is missing and returns
// it absolute and canonical, as the shell's `cd DIR && pwd` gives it. A
// directory that cannot be created, entered or written is fatal, its
// message naming the directory in native form.
func resolveOutDir(outDir string, getwd func() (string, error)) (string, error) {
	if !isDirectory(outDir) {
		if err := osfile.MkdirAll(outDir, 0o777); err != nil || outDir == "" {
			return "", &fatalError{message: "Cannot create output dir: " + outDir}
		}
	}
	cwd := ""
	if !filepath.IsAbs(outDir) {
		var err error
		if cwd, err = getwd(); err != nil {
			return "", &fatalError{message: "cd: can't cd to " + outDir}
		}
	}
	// Resolved as -r is, so a rooted or drive-relative directory on Windows
	// lands on the right drive.
	absolute, _ := absoluteRoot(cwd, outDir)
	absolute = syspath.FromNative(absolute)
	if !canAccess(absolute, accessEnter) {
		return "", &fatalError{message: "cd: can't cd to " + outDir}
	}
	if !outDirWritable(absolute) {
		return "", &fatalError{message: "Output dir not writable: " + syspath.ToNative(absolute)}
	}
	return absolute, nil
}

// resolvePaths resolves the -p paths, then the root, then respells the -p
// paths against the root, in that order, so the first error reported is a
// -p one.
func resolvePaths(options Options, getwd func() (string, error)) (Options, error) {
	options, err := ResolveProjects(options, getwd)
	if err == nil {
		options, err = ResolveRoot(options, getwd)
	}
	if err == nil {
		options.Projects = respellProjects(options.Projects, options.lexicalRoot, options.Root)
	}
	return options, err
}

// probeWrite reports whether a new file can be created in dir, by creating
// a probe file with a random name that must not exist yet and removing it.
func probeWrite(dir string) bool {
	random := make([]byte, 16)
	_, _ = rand.Read(random)
	probe := dir + "/.write-probe-" + hex.EncodeToString(random)
	file, err := osfile.OpenFile(probe, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false
	}
	_ = file.Close()
	_ = osfile.Remove(probe)
	return true
}

// setOutputNames names the run's outputs after the host and the start time.
func (state *runState) setOutputNames() {
	state.hostname = hostname()
	state.name = safeHostname(state.hostname) + "_" + state.started.Format("20060102T150405Z") +
		"_agent-artifacts"
	base := state.options.OutDir + "/"
	state.outDir = state.options.OutDir
	state.stageDir = base + ".stage-" + state.name
	state.archivePath = base + state.name + ".tar.gz"
	state.partialPath = state.archivePath + ".partial"
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

// openOutputs creates the staging directory, the log and the manifest, and
// the collector every walk shares. Under
// --inventory nothing is created: the log goes to the null device and the
// rows nowhere.
func (state *runState) openOutputs() error {
	logPath, rows := os.DevNull, io.Writer(io.Discard)
	physical := ""
	if !state.options.Inventory {
		if err := osfile.MkdirAll(state.stageDir+"/fs", 0o777); err != nil {
			return &fatalError{message: "Cannot create staging dir"}
		}
		state.stageFoldsCase = state.keepCaseApart(state.stageDir + "/fs")
		var err error
		if physical, err = stage.PhysicalDir(state.stageDir); err != nil {
			return &fatalError{message: "Cannot create staging dir"}
		}
		if state.manifestFile, err = osfile.Create(state.manifestPath); err != nil {
			return &fatalError{message: err.Error()}
		}
		state.rows = bufio.NewWriter(nativeErrorWriter{state.manifestFile})
		logPath, rows = state.logPath, state.rows
	}
	log, err := runlog.Open(logPath, state.options.Quiet, state.stderr, state.host.now)
	if err != nil {
		return &fatalError{message: err.Error()}
	}
	state.log, state.manifest = log, manifest.NewWriter(rows)
	state.manifest.UseSameEntry(follow.SameEntry(state.roots.Expansion()))
	state.collector = state.newCollector(physical)
	return nil
}

// keepCaseApart marks dir, the new and empty staging fs directory, so it
// holds two names differing only in case apart (Windows; a no-op elsewhere),
// and reports whether it still takes two names for one (stage.FoldsCase):
// any macOS volume (Unicode forms), an exFAT or FAT drive, or a Windows file
// system without the setting.
func (state *runState) keepCaseApart(dir string) bool {
	mark := state.host.markCaseSensitive
	if mark == nil {
		mark = stage.MarkCaseSensitive
	}
	_ = mark(dir) // whatever the outcome, the probe says what dir does
	return stage.FoldsCase(dir)
}

// foldingLine is the log line of a run whose staging directory takes two
// names that differ in letter case or Unicode form for one.
func foldingLine(outDir string) string {
	return "NOTE: output directory " + syspath.ToNative(outDir) + " ignores letter case or" +
		" Unicode form in names; a file or link whose name it takes for another file's, link's" +
		" or directory's is not staged and its row says why (an output directory on a file" +
		" system that compares names byte for byte keeps both)"
}

// newCollector returns the walk settings of the run: the active exclusions
// (none under --full), the credential test, the staging directory and the
// run's one cache of owner names.
func (state *runState) newCollector(physical string) *walk.Collector {
	excludes := state.tables.Excludes
	if state.options.Full {
		excludes = nil
	}
	root := state.options.Root
	return &walk.Collector{
		Tables: state.tables, Excludes: excludes, Secret: walk.MatchAny(state.tables.SecretGlobs),
		Log: state.log, Manifest: state.manifest, Root: root,
		Stager: stage.Stager{Dir: state.stageDir, Physical: physical, Root: root,
			Opener: state.opener, FoldsCase: state.stageFoldsCase, Places: stage.NewPlaces()},
		MaxSize: state.options.MaxFileBytes, NoSecrets: state.options.NoSecrets,
		Inventory: state.options.Inventory, Records: state.dockerRecords,
		Owners: fsmeta.NewOwners(), RootCheck: state.roots,
	}
}

// mode is the run's mode: image under -r, live otherwise.
func (state *runState) mode() string {
	if state.options.Root != "" {
		return "image"
	}
	return "live"
}

// startLog writes the first log lines: the version, host,
// mode, root and account, the options, on Windows how files are opened, then
// the warnings. The sizes are
// printed from their byte values in whole MiB.
func (state *runState) startLog(context.Context) error {
	system := uname()
	root := syspath.ToNative(state.options.Root)
	if root == "" {
		root = summary.LiveRootText
	}
	options := state.options
	identity := runIdentity()
	lines := []string{
		summary.Tool + " " + version.String() + " starting on " + state.hostname + " (" +
			system.name + " " + system.release + ") mode=" + state.mode() + " root=" + root +
			" " + identityText(identity),
		fmt.Sprintf("hash=crypto/sha256 max_file_size=%dMB full=%s no_secrets=%s no_docker=%s"+
			" env_scan_home=%s env_scan_max_size=%dMB no_preserve_atime=%s",
			options.MaxFileBytes/mebibyte, switchText(options.Full), switchText(options.NoSecrets),
			switchText(options.NoDocker), switchText(options.EnvScanHome), options.EnvScanMaxBytes/mebibyte,
			switchText(options.NoPreserveAtime)),
	}
	if logsAccessDecision {
		lines = append(lines, accessDecisionLine(state.accessUpdates, options.NoPreserveAtime))
	}
	if state.mode() == "live" && !privileged(identity) {
		lines = append(lines, privilegeWarning)
	}
	if options.Inventory && options.OutDirSet {
		lines = append(lines, "NOTE: --inventory writes nothing; -o "+options.OutDir+" ignored")
	}
	if state.stageFoldsCase {
		lines = append(lines, foldingLine(state.outDir))
	}
	for _, line := range lines {
		if err := state.log.Line(line); err != nil {
			return err
		}
	}
	return nil
}

// switchText returns 1 or 0, the form an option takes in the log.
func switchText(set bool) string {
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
		io.WriteString(state.stderr, runlog.Printable(text)+"\n")
	}
}

// confine has the run's opener check, on an image run on Windows, that each
// file or directory it opens lies under the image root's own place
// (follow.Confinement), after the root check let the path through. When
// that place cannot be asked, refusing every open would lose the whole
// image, so the run goes on with the root check alone and one log line
// says so. A live run and a Unix run have no confinement.
func (state *runState) confine(context.Context) error {
	if state.options.Root == "" {
		return nil
	}
	confinement, err := state.host.confine(state.options.Root, state.reportOpenedOutside)
	if err != nil {
		return state.log.Line("image root: where " + syspath.ToNative(state.options.Root) +
			" lies cannot be read (" + err.Error() + "); what is opened under it is checked" +
			" by the root check alone")
	}
	if confinement != nil {
		state.opener.Admit = confinement.Admit
	}
	return nil
}

// reportOpenedOutside logs, once per path (the confinement asks once), a
// path the root check let through that Windows opened outside the image
// root: nothing was read from it.
func (state *runState) reportOpenedOutside(path string) {
	state.line(syspath.ToNative(path) +
		follow.Reason(follow.Refusal{Link: path, Kind: follow.OpenedOutside}, ""))
}

// reportNotHeld logs a file read without the hold on its access time the
// run wanted, with the reason, once per file and reason, so a later and
// worse outcome for the same file is not hidden, and keeps the file for the
// count logNotHeld writes.
func (state *runState) reportNotHeld(path, reason string) {
	state.notHeld[path] = true
	if state.notHeldLines[[2]string{path, reason}] {
		return
	}
	state.notHeldLines[[2]string{path, reason}] = true
	state.line("access time not held: " + syspath.ToNative(path) + ": " + reason)
}

// logNotHeld logs how many distinct files were read without the hold the
// run wanted, when there were any.
func (state *runState) logNotHeld() error {
	if len(state.notHeld) == 0 {
		return nil
	}
	return state.log.Line(fmt.Sprintf("access times not held: %d file(s)", len(state.notHeld)))
}

// cleanup closes the run's files and removes the staging directory unless
// -k was given, on every exit.
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
		_ = osfile.RemoveAll(state.stageDir)
	}
}

// nativeErrorWriter passes writes to the manifest file and returns their
// errors with the file's path in native form; the file was opened through
// osfile, so its own errors name the \\?\ form.
type nativeErrorWriter struct {
	file *os.File
}

// Write writes data to the file.
func (w nativeErrorWriter) Write(data []byte) (int, error) {
	n, err := w.file.Write(data)
	return n, syspath.NativeErrors(err)
}
