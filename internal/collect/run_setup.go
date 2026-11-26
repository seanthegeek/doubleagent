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
	// console writes what the run prints but the inventory (runlog.Console).
	console *runlog.Console
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
	// tree is the staging directory once the run has made it, held open
	// for every staged write (stage.Tree); summaryMade is set once the run
	// has created collection.json, so it removes only what it made.
	tree        *stage.Tree
	summaryMade bool
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
	profiles      []users.Profile
	projects      []projects.Project
	docker        volumes.Counts
	dockerRecords *inventory.Aggregator
	notes         []string
	relocation    relocation.Result
}

// prepareRun resolves the -p paths, the root, the prerequisites, the
// platform's capabilities and the output names. Every failure is a
// fatalError.
func prepareRun(options Options, host environment, stdout io.Writer, console *runlog.Console,
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
	state := &runState{options: options, host: host, stdout: stdout, console: console,
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
// it absolute and canonical, resolved as -r is, so a rooted or drive-relative
// directory on Windows lands on the right drive. Whether the run may write
// in it is not tested: creating the staging directory is the test.
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
			return "", &fatalError{message: "Cannot read the current directory to resolve output dir " +
				outDir + ": " + err.Error()}
		}
	}
	absolute, _ := absoluteRoot(cwd, outDir)
	return syspath.FromNative(absolute), nil
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
// digit, `.`, `_` or `-` with `-`, one `-` per byte.
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
// the collector every walk shares. Each is created new: a name already
// taken, by a link or anything else, is a fatal error and is neither
// followed nor changed. Under --inventory nothing is created and the rows go
// nowhere.
func (state *runState) openOutputs() error {
	rows := io.Writer(io.Discard)
	log := runlog.Discard(state.options.Quiet, state.console, state.host.now)
	if !state.options.Inventory {
		var err error
		if state.tree, err = state.makeStage(); err != nil {
			return err
		}
		if state.manifestFile, err = osfile.CreateNew(state.manifestPath); err != nil {
			return &fatalError{message: "Cannot create manifest: " + err.Error()}
		}
		state.rows = bufio.NewWriter(nativeErrorWriter{state.manifestFile})
		rows = state.rows
		if log, err = runlog.Open(state.logPath, state.options.Quiet, state.console,
			state.host.now); err != nil {
			return &fatalError{message: err.Error()}
		}
	}
	state.log, state.manifest = log, manifest.NewWriter(rows, follow.SameEntry(state.roots.Expansion()))
	state.collector = state.newCollector(state.tree)
	return nil
}

// makeStage creates the staging directory, new and held open, and its fs
// directory through it, marks fs to keep case apart, and returns the tree. A
// failure is a fatalError; a tree made before it is still returned, so
// cleanup removes it.
func (state *runState) makeStage() (*stage.Tree, error) {
	tree, err := stage.CreateTree(state.stageDir)
	if err != nil {
		return nil, &fatalError{message: "Cannot create staging dir: " + err.Error()}
	}
	if err := tree.MkdirAll("fs"); err != nil {
		return tree, &fatalError{message: "Cannot create staging dir: " + err.Error()}
	}
	state.stageFoldsCase = state.keepCaseApart(tree)
	return tree, nil
}

// keepCaseApart marks the new, empty staging fs directory to keep names
// differing only in case apart (Windows only), and reports whether it still
// takes two names for one: any macOS volume (Unicode forms), an exFAT or FAT
// drive, or a Windows file system without the setting.
func (state *runState) keepCaseApart(tree *stage.Tree) bool {
	mark := state.host.markCaseSensitive
	if mark == nil {
		mark = stage.MarkCaseSensitive
	}
	_ = mark(tree.Path() + "/fs") // whatever the outcome, the probe says what fs does
	return tree.FoldsCase("fs")
}

// foldingLine is the log line of a run whose staging directory takes two
// names that differ in letter case or Unicode form for one.
func foldingLine(outDir string) string {
	return "NOTE: output directory " + syspath.ToNative(outDir) + " ignores letter case or" +
		" Unicode form in names; a file or link whose name it takes for another file's, link's" +
		" or directory's is not staged and its row says why (an output directory on a file" +
		" system that compares names byte for byte keeps both)"
}

// newCollector returns the run's walk settings: the active exclusions (none
// under --full), the credential test, the staging tree (nil under
// --inventory) and the run's one cache of owner names.
func (state *runState) newCollector(tree *stage.Tree) *walk.Collector {
	excludes, volumeExcludes := state.tables.Excludes, state.tables.VolumeExcludes
	if state.options.Full {
		excludes, volumeExcludes = nil, nil
	}
	root := state.options.Root
	return &walk.Collector{
		Tables: state.tables, Excludes: excludes, SecretGlobs: state.tables.SecretGlobs,
		VolumeExcludes: volumeExcludes, VolumeSecretGlobs: state.tables.VolumeSecretGlobs,
		Log: state.log, Manifest: state.manifest, Root: root,
		Stager: stage.Stager{Dir: state.stageDir, Tree: tree, Root: root,
			Opener: state.opener, FoldsCase: state.stageFoldsCase, Places: stage.NewPlaces()},
		MaxSize: state.options.MaxFileBytes, NoSecrets: state.options.NoSecrets,
		Inventory: state.options.Inventory, Records: state.dockerRecords,
		Owners: fsmeta.NewOwners(), RootCheck: state.roots, Output: state.output(),
	}
}

// output names the run's own outputs, which the walks pass over without a
// row; under --inventory, which writes nothing, it names none.
func (state *runState) output() stage.RunOutput {
	if state.options.Inventory {
		return stage.RunOutput{}
	}
	return stage.RunOutput{RunName: state.name}
}

// mode is the run's mode: image under -r, live otherwise.
func (state *runState) mode() string {
	if state.options.Root != "" {
		return "image"
	}
	return "live"
}

// startLog writes the first log lines: the version, host, mode, root and
// account, the options (sizes in whole MiB), on Windows how files are opened,
// then the warnings.
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
		fmt.Sprintf("max_file_size=%dMB full=%s no_secrets=%s no_docker=%s"+
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

// line logs text, or, before the log is open, queues it for standard error
// unless the run is quiet. A failed write has nowhere left to be reported.
func (state *runState) line(text string) {
	if state.log != nil {
		_ = state.log.Line(text)
		return
	}
	if !state.options.Quiet {
		state.console.Err(runlog.Printable(text) + "\n")
	}
}

// confine has the opener check, on a Windows image run, that each file or
// directory it opens lies under the image root's own place, after the root
// check let the path through. When that place cannot be read, refusing every
// open would lose the whole image, so the run goes on with the root check
// alone and logs that. Live and Unix runs have no confinement.
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

// reportOpenedOutside logs a path the root check let through that Windows
// opened outside the image root, once (the confinement asks once per path);
// nothing was read from it.
func (state *runState) reportOpenedOutside(path string) {
	state.line(syspath.ToNative(path) +
		follow.Reason(follow.Refusal{Link: path, Kind: follow.OpenedOutside}, ""))
}

// reportNotHeld logs a file read without the hold on its access time the run
// wanted, with the reason, once per file and reason, so a later and worse
// outcome is not hidden, and keeps the file for logNotHeld's count.
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

// cleanup closes the run's files and, on every exit, the staging tree the
// run made, removing it unless -k was given: its contents through the tree,
// then its name only when that still names it. A name that holds something
// else is left, with a log line. Last it finishes the console and closes the
// log (finishConsole).
func (state *runState) cleanup() {
	if state.rows != nil {
		_ = state.rows.Flush()
	}
	if state.manifestFile != nil {
		_ = state.manifestFile.Close()
	}
	if state.tree != nil && state.options.KeepStaging {
		_ = state.tree.Close()
	} else if state.tree != nil {
		if err := state.tree.Discard(); err != nil {
			state.line("staging directory not removed: " + err.Error())
		}
	}
	state.finishConsole()
}

// finishConsole waits a stated time for the console to write what is
// queued, then, when lines were not shown, says how many in the log file
// alone, since the console is what stalled, and closes the log. The count
// tells the responder that the console's copy is short and the log whole.
func (state *runState) finishConsole() {
	notShown := state.console.Finish(state.host.waitForConsole())
	if state.log == nil {
		return
	}
	if notShown > 0 {
		_ = state.log.FileLine(fmt.Sprintf("console: %d lines not shown", notShown))
	}
	_ = state.log.Close()
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
