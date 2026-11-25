package collect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/archive"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/runlog"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/version"
)

// writeArchive writes collection.json, stages it with the manifest and the
// log, has buildArchive write the archive and move it to its name, then logs
// `done` and prints the stdout summary. Once the archive has its name nothing
// looks at ctx or fails, so the run has finished and exits 0 whatever
// arrives: no run exits 130 with an archive in place, or logs `done:` and
// then `interrupted`.
func (state *runState) writeArchive(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	report := state.summary(state.host.now().UTC())
	if err := state.writeCollectionJSON(report); err != nil {
		return err
	}
	if err := state.log.Line("Archiving"); err != nil {
		return err
	}
	if err := state.stageRunFiles(); err != nil {
		return err
	}
	if err := state.buildArchive(ctx, &report); err != nil {
		return err
	}
	state.logDone(report)
	// The summary goes through the console, after the lines before it; a
	// strings.Builder's writes never fail.
	var text strings.Builder
	_ = summary.PrintResult(&text, report)
	state.console.Out(text.String())
	return nil
}

// buildArchive is the archive step, filling the report's hash and size. What
// held the partial or the final name is removed unfollowed where it can be;
// a directory holding entries cannot, and at the partial name, where the
// archive is created new, it ends the run, while at the final name the move
// fails later. The partial archive, hashed as written, takes the final name,
// is removed with its sidecar on a stop request or a failure (an entry put at
// its name meanwhile is left), or stays complete beside its sidecar when the
// move fails or the process is killed. The move is the point of no return:
// ctx is looked at last just before it.
func (state *runState) buildArchive(ctx context.Context, report *summary.Summary) error {
	// Files left at these names by an earlier run are not this run's.
	_ = osfile.Remove(state.archivePath)
	_ = osfile.Remove(state.partialPath)
	written, err := archive.WriteTarGz(ctx, state.tree, state.partialPath)
	ours, sidecarMade := err == nil, false
	if err == nil {
		state.reach(archiveWritten)
		sidecarMade, err = state.recordHash(written, report)
	}
	if err == nil {
		state.reach(sidecarWritten)
		if err = archive.StillNames(state.partialPath, written.File); err != nil {
			ours = false
		}
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		if ours {
			_ = osfile.Remove(state.partialPath)
		}
		if sidecarMade {
			_ = osfile.Remove(state.archivePath + ".sha256")
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return state.archiveNotWritten(err)
	}
	state.moving = true
	return state.placeArchive()
}

// recordHash puts the hash and size of the archive as written into the
// report and writes the sidecar under the archive's final name, reporting
// whether it made the sidecar file.
func (state *runState) recordHash(written archive.Written, report *summary.Summary) (bool, error) {
	report.ArchiveSHA, report.ArchiveSize = written.SHA256, written.Size
	return archive.WriteSidecar(state.archivePath, written.SHA256)
}

// Tries and pauses of the move of the archive to its name: on Windows another
// program (a virus scanner reading the new file, for one) can hold either
// name open for a moment without allowing the rename or the replacement.
const (
	moveAttempts = 5
	movePause    = 250 * time.Millisecond
)

// placeArchive moves the finished archive from its partial name to its name,
// replacing any file there, trying again over about a second; a move that
// fails leaves the archive complete under its partial name with its sidecar,
// and its FATAL line says so. buildArchive has just checked that the partial
// name still names this run's file, but rename(2) and MoveFileEx take names,
// not a handle: an account that may delete entries in the output directory
// can swap in another entry between the two, which then takes the final name
// (a link moved as a link, never followed) and fails the sidecar's hash.
func (state *runState) placeArchive() error {
	rename, sleep := state.host.rename, state.host.sleep
	if rename == nil {
		rename = osfile.Rename
	}
	if sleep == nil {
		sleep = time.Sleep
	}
	var err error
	for attempt := 1; attempt <= moveAttempts; attempt++ {
		if attempt > 1 {
			sleep(movePause)
		}
		if err = rename(state.partialPath, state.archivePath); err == nil {
			state.reach(archiveMoved)
			return nil
		}
	}
	partial := syspath.ToNative(state.partialPath)
	state.line("FATAL: archive not moved to its name: " + err.Error() + "; it is complete as " +
		partial + ", with its .sha256 file beside it")
	return &loggedError{message: "archive not moved to its name"}
}

// archiveNotWritten logs the fatal line of an archive that could not be
// written, with the reason, which names the file, and returns its error.
func (state *runState) archiveNotWritten(reason error) error {
	state.line("FATAL: archive not written: " + reason.Error())
	return &loggedError{message: "archive not written"}
}

// reach tells a test that the archive step has reached point.
func (state *runState) reach(point string) {
	if state.host.reached != nil {
		state.host.reached(point)
	}
}

// The points of the archive step a test can stop a run at (environment.reached).
const (
	archiveWritten = "the archive is written under its partial name"
	sidecarWritten = "the sidecar is written"
	archiveMoved   = "the archive has its name"
)

// logDone appends the `done` line to the log file only, after the copy the
// archive holds, then queues `done` for standard error unless the run is
// quiet. The archive is in place, so the run has finished and a failed write
// is dropped.
func (state *runState) logDone(report summary.Summary) {
	line := fmt.Sprintf("done: %s (%d bytes, sha256 %s)", syspath.ToNative(state.archivePath),
		report.ArchiveSize, report.ArchiveSHA)
	_ = state.log.FileLine(line)
	if !state.options.Quiet {
		state.console.Err("done\n")
	}
}

// summary gathers what collection.json and the stdout summary report, its
// paths in native form.
func (state *runState) summary(finished time.Time) summary.Summary {
	system := uname()
	options := state.options
	report := summary.Summary{
		Version: version.String(), Hostname: state.hostname, Mode: state.mode(),
		Root: options.Root, Platform: system.name + " " + system.release + " " + system.machine,
		Identity: runIdentity(), Started: state.started.Format(runlog.TimeFormat),
		Finished: finished.Format(runlog.TimeFormat),
		Options: summary.Options{Full: options.Full, NoSecrets: options.NoSecrets,
			MaxFileBytes: options.MaxFileBytes, UsersFilter: options.UsersFilter,
			NoDocker: options.NoDocker, NoProjects: options.NoProjects,
			Projects: options.Projects, EnvScanHome: options.EnvScanHome,
			EnvScanMaxSize: options.EnvScanMaxBytes, NoPreserveAtime: options.NoPreserveAtime},
		Plutil: state.plutil.Available(), Counts: state.manifest.Counts(),
		Docker: state.docker, Relocation: state.relocationSummary(), Notes: state.notes,
		Archive:     state.name + ".tar.gz",
		ArchivePath: state.archivePath, ManifestPath: state.manifestPath,
		SummaryPath: state.summaryPath, LogPath: state.logPath,
	}
	for _, home := range state.homes {
		report.Users = append(report.Users, home.User)
		report.Homes = append(report.Homes, home.Path)
	}
	for _, home := range summary.ActiveUsers(state.homes, state.manifest.HasHomeRows) {
		report.ActiveUsers = append(report.ActiveUsers, home.User)
	}
	for _, project := range state.projects {
		// A project the project step leaves unwalked was not collected from.
		if !state.projectNotWalked(project.Path) {
			report.Projects = append(report.Projects, project.Path)
		}
	}
	return nativePaths(report)
}

// nativePaths returns report with every path it prints in native form: the
// root, the homes, the projects, the output files and the file sources of the
// hints. A registry source (HKLM\... or HKU\...) is not a path and is kept.
func nativePaths(report summary.Summary) summary.Summary {
	native := func(paths []string) []string {
		var converted []string
		for _, path := range paths {
			converted = append(converted, syspath.ToNative(path))
		}
		return converted
	}
	report.Root = syspath.ToNative(report.Root)
	report.Homes, report.Projects = native(report.Homes), native(report.Projects)
	report.Options.Projects = native(report.Options.Projects)
	report.ArchivePath = syspath.ToNative(report.ArchivePath)
	report.ManifestPath = syspath.ToNative(report.ManifestPath)
	report.SummaryPath = syspath.ToNative(report.SummaryPath)
	report.LogPath = syspath.ToNative(report.LogPath)
	hints := make([]summary.Hint, 0, len(report.Relocation.Hints))
	for _, hint := range report.Relocation.Hints {
		if !isRegistrySource(hint.Source) {
			hint.Source = syspath.ToNative(hint.Source)
		}
		hints = append(hints, hint)
	}
	report.Relocation.Hints = hints
	return report
}

// isRegistrySource reports whether a hint's source names a registry value,
// as the registry pass writes it, rather than a file.
func isRegistrySource(source string) bool {
	return strings.HasPrefix(source, `HKLM\`) || strings.HasPrefix(source, `HKU\`)
}

// relocationSummary converts the relocation result for the summary, with
// what the registry pass read and, when it read nothing, why: `not Windows`
// off Windows, `image mode` under -r.
func (state *runState) relocationSummary() summary.Relocation {
	result := state.relocation
	registry := result.Registry
	converted := summary.Relocation{RegistryRead: registry.Read, RegistryReason: registry.Reason,
		UsersRead: registry.UsersRead, UsersNotLoaded: registry.UsersNotLoaded,
		UsersDenied: registry.UsersDenied, FilesSearched: result.FilesSearched,
		FilesSkippedSize: result.FilesSkippedSize, Resolved: result.Resolved}
	for _, hint := range result.Hints {
		converted.Hints = append(converted.Hints, summary.Hint{User: hint.User,
			Source: hint.Source, Line: hint.Line, Variable: hint.Variable, Agent: hint.Agent,
			Value: hint.Value})
	}
	return converted
}

// writeCollectionJSON writes collection.json beside the archive and a copy in
// the staging tree, each a file created new (createNew, stage.Tree.CreateNew).
func (state *runState) writeCollectionJSON(report summary.Summary) error {
	var text bytes.Buffer
	if err := summary.WriteCollectionJSON(&text, report); err != nil {
		return err
	}
	file, err := createNew(state.summaryPath)
	state.summaryMade = err == nil
	if err == nil {
		err = writeAll(file, text.Bytes())
	}
	if err == nil {
		file, err = state.tree.CreateNew("collection.json", 0o666)
		if err == nil {
			err = writeAll(file, text.Bytes())
		}
	}
	if err != nil {
		return fmt.Errorf("collection.json not written: %w", err)
	}
	return nil
}

// createNew creates the file at path for writing, new: anything already at
// path, a link or a dangling link included, is an error and is neither
// followed nor changed (O_EXCL; on Windows CREATE_NEW with
// FILE_FLAG_OPEN_REPARSE_POINT, syscall/syscall_windows.go). Every file a
// run writes is made this way, so another account that can write the output
// directory cannot make the run write through a link it planted there.
func createNew(path string) (*os.File, error) {
	return osfile.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
}

// writeAll writes data to file, a file just created new, and closes it. A
// write or close error names the file in native form.
func writeAll(file *os.File, data []byte) error {
	_, err := file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return syspath.NativeErrors(err)
}

// stageRunFiles copies the manifest and the log, as they stand, into the
// staging tree.
func (state *runState) stageRunFiles() error {
	if err := state.rows.Flush(); err != nil {
		return err
	}
	copies := [][2]string{{state.manifestPath, "manifest.jsonl"}, {state.logPath, "collector.log"}}
	for _, files := range copies {
		if err := copyFile(files[0], state.tree, files[1]); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies the file at source to name in the tree, created new
// (stage.Tree.CreateNew) with the default mode the umask narrows. A read,
// write or close error names its file in native form, as the open errors
// do.
func copyFile(source string, tree *stage.Tree, name string) error {
	input, err := osfile.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	out, err := tree.CreateNew(name, 0o666)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, input); err != nil {
		out.Close()
		return syspath.NativeErrors(err)
	}
	return syspath.NativeErrors(out.Close())
}
