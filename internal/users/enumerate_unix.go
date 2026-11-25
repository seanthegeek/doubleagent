//go:build unix

package users

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// isSystemHome reports whether a home, relative to the root, belongs to a
// system or shared profile that a live run never collects: a system path or
// one of five exact paths under /Users.
func isSystemHome(relative string) bool {
	switch relative {
	case "/Users/Shared", "/Users/Public", "/Users/Default", "/Users/All Users",
		"/Users/Default User":
		return true
	}
	return isSystemPath(relative)
}

// namesAnotherPath reports whether a passwd home field would name a path
// other than the image's own: never on Unix, where a backslash is a byte of a
// name like any other.
func namesAnotherPath(string) bool { return false }

// Find returns the homes to collect, in enumeration order: passwd (live,
// getent's, else the file; macOS adds dscl) then imageHomeGlobs. reach
// (walk.RootCheck.Reached; nil: none) first replaces each candidate's dotted
// path by the one the system reaches. usersFilter is the -u list. A live run
// skips by isSystemHome and compares names case-sensitively, an image run
// follows imageRule. A home listed twice keeps its first user; only
// directories are returned, and a candidate whose check fails other than as
// missing is in Skipped. root is "" or "/" live. Once ctx is cancelled no
// program is run. check is the image root's (nil live): nothing behind a
// link out of the root is read, listed or taken as a home.
func Find(ctx context.Context, live bool, root, usersFilter string, run Runner,
	check glob.Check, opener *fsmeta.Opener, reach func(path string) string,
) Report {
	root = strings.TrimSuffix(root, "/")
	guard := &rootGuard{check: check, opener: opener}
	rule := imageRule(root)
	var candidates []Home
	if live {
		rule = homeRule{
			skip:    func(home Home) bool { return isSystemHome(strings.TrimPrefix(home.Path, root)) },
			matches: sameName,
		}
		candidates = append(livePasswdHomes(ctx, root, run, guard), dsclHomes(ctx, run)...)
	} else {
		candidates = passwdFileHomes(ctx, root, guard)
	}
	candidates = guard.inside(append(candidates, globHomes(root, guard)...))
	candidates, written := reachAll(candidates, reach)
	names := userNames(usersFilter, rule.matches)
	report := filterHomes(candidates, written, usersFilter != "", names, guard, rule)
	report.OutOfRoot, report.LongLines = guard.links, guard.longLines
	report.Unmatched = unmatchedUsers(names, report.Found, rule.matches)
	return report
}

// sameName reports whether a -u name names user on a live run: byte for
// byte, since Unix account names are case-sensitive.
func sameName(name, user string) bool { return name == user }

// livePasswdHomes returns the passwd homes (passwdHomes) of getent's output
// when getent runs, else of the file (passwdFileHomes).
func livePasswdHomes(ctx context.Context, root string, run Runner, guard *rootGuard) []Home {
	if run != nil && ctx.Err() == nil {
		if output, err := run(ctx, "getent", "passwd"); err == nil {
			return passwdHomes(root, output)
		}
	}
	return passwdFileHomes(ctx, root, guard)
}

// passwdHomes returns the homes (passwdHome) of the lines of text, a passwd
// file already in memory such as getent's output.
func passwdHomes(root string, text []byte) []Home {
	var homes []Home
	for _, line := range bytes.Split(text, []byte("\n")) {
		if home, ok := passwdHome(root, line); ok {
			homes = append(homes, home)
		}
	}
	return homes
}

// globHomes returns, for each home parent in order, the entries that are
// directories or cannot be examined, each named by its last path segment.
// Each match is one home under its whole name, a newline in it included.
func globHomes(root string, guard *rootGuard) []Home {
	var homes []Home
	for _, pattern := range imageHomeGlobs() {
		for _, match := range guard.expand(root, pattern) {
			if isDir, err := guard.examine(match); isDir || err != nil {
				homes = append(homes, Home{match[strings.LastIndexByte(match, '/')+1:], match})
			}
		}
	}
	return homes
}

// filterHomes returns the report's homes, skipped candidates, found users
// and merges: it drops the homes rule skips, candidates that are not
// directories or do not exist, users the -u list did not name (names under
// rule's comparison, when filtered), and every home already seen. Found
// lists the users of candidates that pass every test but -u's. written holds
// each candidate's path as it was written, before reaching. A repeated home
// that is a directory is a merge, and is reported, when its two written
// paths differ.
func filterHomes(candidates []Home, written []string, filtered bool, names []string,
	guard *rootGuard, rule homeRule,
) (report Report) {
	seen := map[string]int{}
	for i, home := range candidates {
		if rule.skip(home) {
			continue
		}
		isDir, err := guard.examine(home.Path)
		if !isDir && err == nil {
			continue
		}
		report.Found = addFound(report.Found, home.User)
		if filtered && !named(names, home.User, rule.matches) {
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
