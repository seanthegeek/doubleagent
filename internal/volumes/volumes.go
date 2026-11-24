package volumes

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/syspath"
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

// Bases are where the system volume directories are looked for, in
// canonical form: System holds var/lib, ProgramData the Docker for Windows
// data. In image mode they are the root and root/ProgramData; in a live run
// "" and /ProgramData on Unix, the system drive and the ProgramData
// directory the environment names on Windows.
type Bases struct {
	System      string
	ProgramData string
}

// collection is the state of one Collect call, with the links out of the
// image root already logged.
type collection struct {
	collector *walk.Collector
	notes     *[]string
	counts    Counts
	outOfRoot map[string]bool
}

// Collect collects the volumes of the system and per-user Docker and Podman
// data roots under bases and of every home in homes, and notes Docker
// Desktop data. Notes are
// appended to notes and logged, with their paths in native form. Under
// an inventory walk it records each matched volume through collector.Records
// and writes no row. The error is a cancelled context or a failed log or
// manifest write; the counts are those reached so far.
func Collect(ctx context.Context, collector *walk.Collector, bases Bases, homes []users.Home,
	notes *[]string,
) (Counts, error) {
	run := &collection{collector: collector, notes: notes, outOfRoot: map[string]bool{}}
	err := run.collectRoots(ctx, bases, homes)
	if err != nil {
		return run.counts, err
	}
	return run.counts, run.noteDesktop(desktopDirs(bases, homes))
}

// collectRoots tries the system volume directories, then each home's.
func (run *collection) collectRoots(ctx context.Context, bases Bases, homes []users.Home) error {
	system := []struct{ base, rel string }{
		{bases.System + "/var/lib", "docker/volumes"},
		{bases.System + "/var/lib", "containers/storage/volumes"},
		{bases.ProgramData, "Docker/volumes"},
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
func desktopDirs(bases Bases, homes []users.Home) []string {
	dirs := []string{bases.ProgramData + "/DockerDesktop"}
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
		leaves, err := run.leavesRoot(dir)
		if err != nil {
			return err
		}
		if leaves {
			continue
		}
		meta, err := fsmeta.Lstat(dir)
		if err != nil || meta.Kind != fsmeta.Dir {
			continue
		}
		run.counts.Seen, run.counts.Desktop = true, true
		note := "docker: Docker Desktop data at " + syspath.ToNative(dir) + ";"
		if err := run.addNote(note + " volumes inside its virtual machine disk are not collected"); err != nil {
			return err
		}
	}
	return nil
}

// leavesRoot reports whether path, or a directory above it, is a link out of
// the image root (collector.RootCheck; never on a live run), logging the link
// once: nothing behind it is stat'ed or listed, so no volume or Docker
// Desktop directory of the analyst's host is reported.
func (run *collection) leavesRoot(path string) (bool, error) {
	refusal, leaves := run.collector.RootCheck.Exit(path)
	if !leaves || run.outOfRoot[refusal.Link] {
		return leaves, nil
	}
	run.outOfRoot[refusal.Link] = true
	reason := follow.Reason(refusal, " leads out of the image root; nothing behind it was listed")
	return true, run.collector.Log.Line("docker: " + syspath.ToNative(refusal.Link) + reason)
}

// addNote records a note and logs it with the NOTE prefix.
func (run *collection) addNote(note string) error {
	*run.notes = append(*run.notes, note)
	return run.collector.Log.Line("NOTE: " + note)
}

// volumeRoot checks every component of base/rel. It
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
			note := "docker: " + syspath.ToNative(dir) +
				" is a symlink and was not followed; collect its target by hand"
			return dir, false, run.addNote(note)
		}
		// A component the root check refuses (on Windows a reparse point
		// other than a symlink or junction, which lstat does not call a
		// symlink) gets its line once, as a base does.
		if leaves, err := run.leavesRoot(dir); leaves || err != nil {
			return dir, false, err
		}
		if !run.isDirectory(dir) {
			return dir, false, nil
		}
		last := index == len(parts)-1
		readable, refused := true, false
		if last {
			readable, refused = run.canRead(dir)
		}
		if refused {
			return dir, false, nil
		}
		if !run.canEnter(dir) || !readable {
			run.counts.Seen = true
			run.counts.Unreadable++
			note := "docker: " + syspath.ToNative(dir) +
				" is not readable; run as root to collect the volumes under it"
			return dir, false, run.addNote(note)
		}
	}
	return dir, true, nil
}

// splitComponents splits a relative path into its non-empty components.
func splitComponents(rel string) []string {
	return slices.DeleteFunc(strings.Split(rel, "/"), func(part string) bool { return part == "" })
}

// isDirectory reports whether target is, or links to, a directory, as test
// -d; never for a target out of the image root, which is not followed.
func (run *collection) isDirectory(target string) bool {
	isDir, _ := follow.IsDirectory(run.collector.RootCheck.Expansion(), target)
	return isDir
}

// canRead reports whether the directory target can be listed; never for a
// target out of the image root, which is not followed. refused is set for a
// directory the opener refused because Windows opened it outside the image
// root: logged once already, it is neither readable nor unreadable.
func (run *collection) canRead(target string) (readable, refused bool) {
	readable, err := follow.CanRead(run.collector.RootCheck.Expansion(),
		run.collector.Stager.Opener, target)
	return readable, follow.IsOpenedOutside(err)
}

// canEnter reports whether the directory target may be entered; never for a
// target out of the image root, which is not followed.
func (run *collection) canEnter(target string) bool {
	enterable, _ := follow.CanEnter(run.collector.RootCheck.Expansion(), target)
	return enterable
}

// collectVolumeDir collects every volume under base/rel. A volume that is a
// symlink or not a directory is skipped uncounted.
func (run *collection) collectVolumeDir(ctx context.Context, base, rel string) error {
	// A link inside rel keeps volumeRoot's note; one at or above base, which
	// volumeRoot does not look at, is out of the root's reach.
	if leaves, err := run.leavesRoot(base); leaves || err != nil {
		return err
	}
	dir, ok, err := run.volumeRoot(base, rel)
	if err != nil || !ok {
		return err
	}
	run.counts.Seen = true
	if err := run.collector.Log.Line("Docker volumes in " + syspath.ToNative(dir)); err != nil {
		return err
	}
	// dir passed the root check above, so the check refuses nothing here.
	volumes, _ := glob.Expand(dir, "*", run.collector.RootCheck.Expansion(),
		run.collector.Stager.Opener)
	for _, volume := range volumes {
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

// volumeRule returns the DOCKER_VOLUMES line whose glob matches name and is
// the most specific: fewest `*`, then most literal characters, then earliest
// in the table, which catalog.Parse sorts by agent and then glob, so of two
// equally specific lines the one whose agent sorts first wins.
func volumeRule(table []catalog.Entry, name string) (catalog.Entry, bool) {
	best, bestStars, bestLiterals := -1, 0, 0
	for i, entry := range table {
		if !glob.MatchPath(entry.Glob, name) {
			continue
		}
		stars, literals := globSpecificity(entry.Glob)
		if best < 0 || stars < bestStars || (stars == bestStars && literals > bestLiterals) {
			best, bestStars, bestLiterals = i, stars, literals
		}
	}
	if best < 0 {
		return catalog.Entry{}, false
	}
	return table[best], true
}

// globSpecificity returns the number of `*` wildcards in pattern and the
// number of characters it matches literally. A `?` or a bracket list such as
// `[a-z]` or `[!.]` matches one character of a set and counts as neither,
// and a `*` or `?` inside a bracket list is a member of the set, not a
// wildcard. The table's globs hold no backslash and every bracket is closed
// (internal/catalog/AGENTS.md, "Glob forms").
func globSpecificity(pattern string) (stars, literals int) {
	for position := 0; position < len(pattern); position++ {
		switch pattern[position] {
		case '*':
			stars++
		case '?':
		case '[':
			position = bracketEnd(pattern, position)
		default:
			literals++
		}
	}
	return stars, literals
}

// bracketEnd returns the index of the `]` that closes the bracket list
// opening at open, where a `]` first in the list (after any `!`) is a member.
// An unclosed list, which MatchPath never matches, ends the count.
func bracketEnd(pattern string, open int) int {
	position := open + 1
	if position < len(pattern) && pattern[position] == '!' {
		position++
	}
	if position < len(pattern) && pattern[position] == ']' {
		position++
	}
	closing := strings.IndexByte(pattern[position:], ']')
	if closing < 0 {
		return len(pattern)
	}
	return position + closing
}

// volumeUnreadable reports whether the volume directory, or its _data
// directory when that is one, cannot be listed and entered; refused when the
// opener refused one of them (canRead).
func (run *collection) volumeUnreadable(volume, data string) (unreadable, refused bool) {
	readable, refused := run.canRead(volume)
	if refused || !readable || !run.canEnter(volume) {
		return !refused, refused
	}
	if !run.isDirectory(data) {
		return false, false
	}
	readable, refused = run.canRead(data)
	return !refused && (!readable || !run.canEnter(data)), refused
}

// collectVolume collects one volume's _data as user docker, or records an
// unmatched or unreadable one. A _data that is a link is skipped on its
// lstat alone, before anything would follow it, so the outcome never depends
// on what the link points at.
func (run *collection) collectVolume(ctx context.Context, volume string) error {
	name := path.Base(volume)
	data := volume + "/_data"
	rule, matched := volumeRule(run.collector.Tables.DockerVolumes, name)
	meta, dataError := fsmeta.Lstat(data)
	isLink := dataError == nil && meta.IsLink()
	unreadable, refused := false, false
	if !isLink {
		unreadable, refused = run.volumeUnreadable(volume, data)
	}
	switch {
	case isLink:
		return run.collector.Log.Line("  volume " + name + " has no _data directory, skipped")
	case refused:
		// Windows opened it outside the image root: logged once by the
		// opener's Admit, and nothing in it is used.
		return nil
	case unreadable:
		run.counts.Unreadable++
		label := "unmatched"
		if matched {
			label = rule.Agent
		}
		return run.addNote("docker: volume " + name + " (" + label + ") at " +
			syspath.ToNative(volume) + " is not readable; run as root to collect it")
	case dataError != nil || !run.isDirectory(data):
		return run.collector.Log.Line("  volume " + name + " has no _data directory, skipped")
	case !matched && run.collector.Inventory:
		return nil
	case !matched:
		return run.writeUnmatched(ctx, data, meta)
	}
	return run.collectMatched(ctx, name, data, rule)
}

// writeUnmatched writes the one dir row of a volume no table line matches, so
// it stays visible, with the size of the files under it, or ctx's error when
// ctx is cancelled while the volume is being sized. Its times and owner are
// empty, and on Windows its attributes are those of the _data directory
// (meta).
func (run *collection) writeUnmatched(ctx context.Context, data string, meta fsmeta.Meta) error {
	size, err := fsmeta.DiskUsageBytes(ctx, run.collector.Stager.Opener, data)
	if err != nil {
		return err
	}
	return run.collector.Manifest.Write(manifest.Row{
		User: dockerUser, Home: data, Path: data, Type: "dir",
		Size: size, Attributes: fsmeta.AttributesString(meta.Attributes),
		Status: manifest.SkippedUnmatchedVolume,
	})
}

// collectMatched walks a matched volume as a home of user docker. Directories
// the walk could not read make the volume count as unreadable.
func (run *collection) collectMatched(ctx context.Context, name, data string,
	rule catalog.Entry,
) error {
	collector := run.collector
	if err := collector.Log.Line("  [" + rule.Agent + "] " + syspath.ToNative(data)); err != nil {
		return err
	}
	if collector.Inventory {
		collector.Records.Match(rule.Agent, rule.Glob)
	}
	unreadable, err := collector.CollectVolume(ctx, dockerUser, rule.Agent, data)
	if err != nil {
		return fmt.Errorf("collect volume %s: %w", syspath.ToNative(data), err)
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
