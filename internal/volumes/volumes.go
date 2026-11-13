// Package volumes finds and collects Docker and Podman named volumes from the
// filesystem, as the sh collector's collect_docker_volumes does, and notes
// Docker Desktop data whose volumes sit inside a virtual machine disk.
// collectors/docs/docker.md has the rules.
package volumes

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// dockerUser is the user name volume rows carry.
const dockerUser = "docker"

// Counts is what the summary reports about volumes. Seen is true once any
// volume directory or Docker Desktop data was met, which is when the stdout
// summary prints its docker line.
type Counts struct {
	Seen       bool
	Found      int
	Collected  int
	Unreadable int
	Desktop    bool
}

// collection is the state of one Collect call.
type collection struct {
	collector *walk.Collector
	notes     *[]string
	counts    Counts
}

// Collect collects the volumes of the system and per-user Docker and Podman
// data roots under root ("" for the live system) and of every home in homes,
// and notes Docker Desktop data. Notes are appended to notes and logged. Under
// an inventory walk it records each matched volume through collector.Records
// and writes no row. The error is a cancelled context or a failed log or
// manifest write; the counts are those reached so far.
func Collect(ctx context.Context, collector *walk.Collector, root string, homes []users.Home,
	notes *[]string,
) (Counts, error) {
	run := &collection{collector: collector, notes: notes}
	err := run.collectRoots(ctx, root, homes)
	if err != nil {
		return run.counts, err
	}
	return run.counts, run.noteDesktop(desktopDirs(root, homes))
}

// collectRoots tries the system volume directories, then each home's.
func (run *collection) collectRoots(ctx context.Context, root string, homes []users.Home) error {
	system := []struct{ base, rel string }{
		{root + "/var/lib", "docker/volumes"},
		{root + "/var/lib", "containers/storage/volumes"},
		{root + "/ProgramData", "Docker/volumes"},
	}
	for _, spec := range system {
		if err := run.collectVolumeDir(ctx, spec.base, spec.rel); err != nil {
			return err
		}
	}
	for _, home := range homes {
		if home.Path == "" {
			continue
		}
		for _, rel := range []string{"docker/volumes", "containers/storage/volumes"} {
			if err := run.collectVolumeDir(ctx, home.Path+"/.local/share", rel); err != nil {
				return err
			}
		}
	}
	return nil
}

// desktopDirs lists where Docker Desktop keeps its data: the system one, then
// two per home.
func desktopDirs(root string, homes []users.Home) []string {
	dirs := []string{root + "/ProgramData/DockerDesktop"}
	for _, home := range homes {
		if home.Path == "" {
			continue
		}
		dirs = append(dirs, home.Path+"/Library/Containers/com.docker.docker",
			home.Path+"/AppData/Local/Docker")
	}
	return dirs
}

// noteDesktop adds a note for each Docker Desktop directory that exists and is
// not a symlink, and marks the run as having Docker Desktop data.
func (run *collection) noteDesktop(dirs []string) error {
	for _, dir := range dirs {
		meta, err := fsmeta.Lstat(dir)
		if err != nil || meta.Kind != fsmeta.Dir {
			continue
		}
		run.counts.Seen, run.counts.Desktop = true, true
		note := "docker: Docker Desktop data at " + dir + ";"
		if err := run.addNote(note + " volumes inside its virtual machine disk are not collected"); err != nil {
			return err
		}
	}
	return nil
}

// addNote records a note and logs it with the NOTE prefix.
func (run *collection) addNote(note string) error {
	*run.notes = append(*run.notes, note)
	return run.collector.Log.Line("NOTE: " + note)
}

// volumeRoot checks every component of base/rel, as test_volume_root does. It
// returns the directory and true when each can be entered and the last listed.
// A symlink or an unreadable component gets a note, and the volumes under it
// are not collected.
func (run *collection) volumeRoot(base, rel string) (string, bool, error) {
	dir := base
	parts := splitComponents(rel)
	for index, part := range parts {
		dir = dir + "/" + part
		meta, err := fsmeta.Lstat(dir)
		if err == nil && meta.Kind == fsmeta.Symlink {
			run.counts.Seen = true
			note := "docker: " + dir + " is a symlink and was not followed; collect its target by hand"
			return dir, false, run.addNote(note)
		}
		if !isDirectory(dir) {
			return dir, false, nil
		}
		last := index == len(parts)-1
		if !canEnter(dir) || (last && !canRead(dir)) {
			run.counts.Seen = true
			run.counts.Unreadable++
			note := "docker: " + dir + " is not readable; run as root to collect the volumes under it"
			return dir, false, run.addNote(note)
		}
	}
	return dir, true, nil
}

// splitComponents splits a relative path into its non-empty components.
func splitComponents(rel string) []string {
	var parts []string
	start := 0
	for index := 0; index <= len(rel); index++ {
		if index == len(rel) || rel[index] == '/' {
			if index > start {
				parts = append(parts, rel[start:index])
			}
			start = index + 1
		}
	}
	return parts
}

// isDirectory reports whether target is, or links to, a directory, as test -d.
func isDirectory(target string) bool {
	info, err := os.Stat(target)
	return err == nil && info.IsDir()
}

// collectVolumeDir collects every volume under base/rel, as collect_volume_dir
// does. A volume that is a symlink or not a directory is skipped uncounted.
func (run *collection) collectVolumeDir(ctx context.Context, base, rel string) error {
	dir, ok, err := run.volumeRoot(base, rel)
	if err != nil || !ok {
		return err
	}
	run.counts.Seen = true
	if err := run.collector.Log.Line("Docker volumes in " + dir); err != nil {
		return err
	}
	for _, volume := range glob.Expand(dir, "*") {
		if err := ctx.Err(); err != nil {
			return err
		}
		meta, err := fsmeta.Lstat(volume)
		if err != nil || meta.Kind != fsmeta.Dir {
			continue
		}
		run.counts.Found++
		if err := run.collectVolume(ctx, volume); err != nil {
			return err
		}
	}
	return nil
}

// volumeRule returns the first DOCKER_VOLUMES line whose glob matches name,
// as get_docker_volume_rule does.
func volumeRule(table []catalog.Entry, name string) (catalog.Entry, bool) {
	for _, entry := range table {
		if glob.MatchPath(entry.Glob, name) {
			return entry, true
		}
	}
	return catalog.Entry{}, false
}

// volumeUnreadable reports whether the volume directory, or its _data
// directory when that is one, cannot be listed and entered.
func volumeUnreadable(volume, data string) bool {
	if !canRead(volume) || !canEnter(volume) {
		return true
	}
	return isDirectory(data) && (!canRead(data) || !canEnter(data))
}

// collectVolume collects one volume's _data as user docker, or records an
// unmatched or unreadable one, as collect_volume does.
func (run *collection) collectVolume(ctx context.Context, volume string) error {
	name := path.Base(volume)
	data := volume + "/_data"
	rule, matched := volumeRule(run.collector.Tables.DockerVolumes, name)
	meta, dataError := fsmeta.Lstat(data)
	switch {
	case volumeUnreadable(volume, data):
		run.counts.Unreadable++
		label := "unmatched"
		if matched {
			label = rule.Agent
		}
		return run.addNote("docker: volume " + name + " (" + label + ") at " + volume +
			" is not readable; run as root to collect it")
	case dataError != nil || meta.Kind == fsmeta.Symlink || !isDirectory(data):
		return run.collector.Log.Line("  volume " + name + " has no _data directory, skipped")
	case !matched && run.collector.Inventory:
		return nil
	case !matched:
		return run.writeUnmatched(data)
	}
	return run.collectMatched(ctx, name, data, rule)
}

// writeUnmatched writes the one dir row of a volume no table line matches, so
// it stays visible, with the size du -sk would report.
func (run *collection) writeUnmatched(data string) error {
	return run.collector.Manifest.Write(manifest.Row{
		User: dockerUser, Home: data, Path: data, Type: "dir",
		Size: fsmeta.DiskUsageBytes(data), Mode: "0",
		Status: manifest.SkippedUnmatchedVolume,
	})
}

// collectMatched walks a matched volume as a home of user docker. Directories
// the walk could not read make the volume count as unreadable.
func (run *collection) collectMatched(ctx context.Context, name, data string,
	rule catalog.Entry,
) error {
	collector := run.collector
	if err := collector.Log.Line("  [" + rule.Agent + "] " + data); err != nil {
		return err
	}
	if collector.Inventory {
		collector.Records.Match(rule.Agent, rule.Glob)
	}
	unreadable, err := collector.CollectPath(ctx, dockerUser, data, rule.Agent, data)
	if err != nil {
		return fmt.Errorf("collect volume %s: %w", data, err)
	}
	run.counts.Collected++
	if unreadable > 0 && !collector.Inventory {
		run.counts.Unreadable++
		note := "docker: volume " + name + " (" + rule.Agent + ") could not be walked completely (" +
			strconv.Itoa(unreadable) + " unreadable paths, see collector.log)"
		return run.addNote(note + "; run as root to collect it")
	}
	return nil
}
