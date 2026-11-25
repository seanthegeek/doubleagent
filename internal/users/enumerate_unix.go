//go:build unix

package users

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// homeGlobs returns the usual home parents, in the order they are tried,
// each relative to the root.
func homeGlobs() []string {
	return []string{"home/*", "Users/*", "root", "var/root", "usr/home/*", "export/home/*"}
}

// isSystemHome reports whether a home, relative to the root, belongs to a
// system or shared profile that is never collected.
func isSystemHome(relative string) bool {
	switch relative {
	case "", "/", "/bin", "/sbin", "/usr", "/usr/bin", "/usr/sbin", "/dev", "/dev/null",
		"/proc", "/sys", "/nonexistent", "/var/empty", "/Users/Shared", "/Users/Public",
		"/Users/Default", "/Users/All Users", "/Users/Default User":
		return true
	}
	return false
}

// Find returns the homes to collect, in enumeration order: passwd (live,
// getent's, else the file; macOS adds dscl) then the usual home parents.
// reach (walk.RootCheck.Reached; nil: none) first replaces each candidate's
// dotted path by the one the system reaches. usersFilter is the -u list,
// compared case-sensitively; a home listed twice keeps its first user; only
// directories are returned, and a candidate whose check fails other than as
// missing is in Skipped. root is "" or "/" live. Once ctx is cancelled no
// program is run. check is the image root's (nil live): nothing behind a
// link out of the root is read, listed or taken as a home.
func Find(ctx context.Context, live bool, root, usersFilter string, run Runner,
	check glob.Check, opener *fsmeta.Opener, reach func(path string) string,
) Report {
	root = strings.TrimSuffix(root, "/")
	guard := &rootGuard{check: check, opener: opener}
	candidates := passwdHomes(ctx, live, root, run, guard)
	if live {
		candidates = append(candidates, dsclHomes(ctx, run)...)
	}
	candidates = guard.inside(append(candidates, globHomes(root, guard)...))
	candidates, written := reachAll(candidates, reach)
	names := userNames(usersFilter, sameName)
	report := filterHomes(candidates, written, root, usersFilter != "", names, guard)
	report.OutOfRoot = guard.links
	report.Unmatched = unmatchedUsers(names, report.Found, sameName)
	return report
}

// sameName reports whether a -u name names user: byte for byte, since Unix
// account names are case-sensitive.
func sameName(name, user string) bool { return name == user }

// passwdHomes returns the user and root-prefixed home of every passwd line
// with at least six fields and a non-empty sixth. Live, the text is
// getent's when getent runs, else the file's.
func passwdHomes(ctx context.Context, live bool, root string, run Runner, guard *rootGuard,
) []Home {
	var text []byte
	ran := false
	if live && run != nil && ctx.Err() == nil {
		output, err := run(ctx, "getent", "passwd")
		text, ran = output, err == nil
	}
	if !ran {
		// A link to the file is followed; a directory or missing file gives no text.
		var err error
		text, err = follow.ReadFile(guard.check, root+"/etc/passwd")
		if refusal, out := follow.OutOfRoot(err); out {
			guard.record(refusal)
		}
	}
	var homes []Home
	for _, line := range bytes.Split(text, []byte("\n")) {
		fields := strings.Split(string(line), ":")
		if len(fields) >= 6 && fields[5] != "" {
			homes = append(homes, Home{fields[0], root + fields[5]})
		}
	}
	return homes
}

// dsclHomes returns the user and home of each line of `dscl . -list /Users
// NFSHomeDirectory`: the first two blank-separated fields, so a home with a
// space in it is cut at the space.
func dsclHomes(ctx context.Context, run Runner) []Home {
	if run == nil || ctx.Err() != nil {
		return nil
	}
	output, err := run(ctx, "dscl", ".", "-list", "/Users", "NFSHomeDirectory")
	if err != nil {
		return nil
	}
	var homes []Home
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.FieldsFunc(line, isBlank)
		if len(fields) >= 2 {
			homes = append(homes, Home{fields[0], fields[1]})
		}
	}
	return homes
}

// isBlank reports whether char separates the fields of a dscl line: a space,
// a tab or a newline.
func isBlank(char rune) bool { return char == ' ' || char == '\t' || char == '\n' }

// globHomes returns, for each home parent in order, the entries that are
// directories or cannot be examined, each named by its last path segment.
// Each match is one home under its whole name, a newline in it included.
func globHomes(root string, guard *rootGuard) []Home {
	var homes []Home
	for _, pattern := range homeGlobs() {
		for _, match := range guard.expand(root, pattern) {
			if isDir, err := guard.examine(match); isDir || err != nil {
				homes = append(homes, Home{match[strings.LastIndexByte(match, '/')+1:], match})
			}
		}
	}
	return homes
}

// filterHomes returns the report's homes, skipped candidates, found users
// and merges: it drops system and shared profile homes (compared without the
// root), candidates that are not directories or do not exist, users the -u
// list did not name (names, when filtered; a list naming nobody keeps no
// home), and every home already seen. Found lists the users of candidates
// that pass every test but -u's. written holds each candidate's path as it
// was written, before reaching. A repeated home is a merge, and is reported,
// when its two written paths differ.
func filterHomes(candidates []Home, written []string, root string, filtered bool,
	names []string, guard *rootGuard,
) (report Report) {
	seen := map[string]int{}
	for i, home := range candidates {
		if isSystemHome(strings.TrimPrefix(home.Path, root)) {
			continue
		}
		isDir, err := guard.examine(home.Path)
		if !isDir && err == nil {
			continue
		}
		report.Found = addFound(report.Found, home.User)
		if filtered && !slices.Contains(names, home.User) {
			continue
		}
		if first, repeated := seen[home.Path]; repeated {
			// Only a directory is a home Find returns and walks once.
			if written[first] != written[i] && isDir {
				report.Merged = append(report.Merged, Merge{Path: home.Path,
					Kept: candidates[first].User, Dropped: home.User})
			}
			continue
		}
		seen[home.Path] = i
		if isDir {
			report.Homes = append(report.Homes, home)
		} else {
			report.Skipped = append(report.Skipped, Skipped{User: home.User, Path: home.Path,
				Reason: err.Error(), Denied: parentLists(home.Path)})
		}
	}
	return report
}

// examine reports whether path is a directory or a link to one, or the
// error that kept it from being examined. A path that does not exist (also
// one that runs through a regular file), or leads out of the image root (its
// link recorded, never followed), is no directory and no error.
func (guard *rootGuard) examine(path string) (bool, error) {
	info, err := follow.Stat(guard.check, path)
	if refusal, out := follow.OutOfRoot(err); out {
		guard.record(refusal)
		return false, nil
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return false, nil
	}
	return err == nil && info.IsDir(), err
}
