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
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/version"
)

// writeArchive writes collection.json, stages it with the manifest and the
// log, archives the staging directory, hashes the archive and prints the
// stdout summary.
func (state *runState) writeArchive(ctx context.Context) error {
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
	_ = osfile.Remove(state.archivePath)
	if err := archive.WriteTarGz(state.stageDir, state.archivePath); err != nil {
		state.line("archive: " + err.Error())
		state.line("FATAL: archive not written")
		return &loggedError{message: "archive not written"}
	}
	if err := state.hashArchive(&report); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// The archive and the log's done line are written; a summary that
	// cannot reach a console that went away is dropped.
	_ = summary.PrintResult(state.stdout, report)
	return nil
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
// the staging directory.
func (state *runState) writeCollectionJSON(report summary.Summary) error {
	var text bytes.Buffer
	if err := summary.WriteCollectionJSON(&text, report); err != nil {
		return err
	}
	for _, path := range []string{state.summaryPath, state.stageDir + "/collection.json"} {
		if err := osfile.WriteFile(path, text.Bytes(), 0o666); err != nil {
			return err
		}
	}
	return nil
}

// stageRunFiles copies the manifest and the log, as they stand, into the
// staging directory.
func (state *runState) stageRunFiles() error {
	if err := state.rows.Flush(); err != nil {
		return err
	}
	copies := [][2]string{{state.manifestPath, "/manifest.jsonl"}, {state.logPath, "/collector.log"}}
	for _, files := range copies {
		if err := copyFile(files[0], state.stageDir+files[1]); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies the file at source to target, created with the default
// mode the umask narrows. A read, write or close error names its file in
// native form, as the open errors do.
func copyFile(source, target string) error {
	input, err := osfile.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	out, err := osfile.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, input); err != nil {
		out.Close()
		return syspath.NativeErrors(err)
	}
	return syspath.NativeErrors(out.Close())
}

// hashArchive hashes the archive, writes its sidecar file and appends the
// `done` line to the log file only, after the copy the archive holds, then
// says `done` on standard error unless the run is quiet, dropping that
// write's failure as the log does.
func (state *runState) hashArchive(report *summary.Summary) error {
	sha256, size, err := archive.SHA256File(state.archivePath)
	if err != nil || size == 0 {
		state.line("FATAL: archive not written")
		return &loggedError{message: "archive not written"}
	}
	report.ArchiveSHA, report.ArchiveSize = sha256, size
	if err := archive.WriteSidecar(state.archivePath, sha256); err != nil {
		return err
	}
	line := fmt.Sprintf("done: %s (%d bytes, sha256 %s)", syspath.ToNative(state.archivePath),
		size, sha256)
	if err := state.log.FileLine(line); err != nil {
		return err
	}
	if !state.options.Quiet {
		_, _ = io.WriteString(state.stderr, "done\n")
	}
	return nil
}
