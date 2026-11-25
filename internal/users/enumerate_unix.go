//go:build unix

package users

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/syspath"
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
// follows imageRule. A home listed twice keeps its first user (filterHomes);
// only directories are returned, and a candidate whose check fails other
// than as missing is in Skipped. root is "" or "/" live. Once ctx is
// cancelled no program is run. check is the image root's (nil live): nothing
// behind a link out of the root is read, listed or taken as a home.
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
	listed := globHomes(root, guard)
	candidates = guard.inside(candidates)
	firstListed := len(candidates)
	candidates, written := reachAll(append(candidates, guard.inside(listed)...), reach)
	names := userNames(usersFilter, rule.matches)
	report := filterHomes(candidates, written, firstListed, usersFilter != "", names, guard, rule)
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

// dsclHomes returns the user and home of each line of `dscl . -list /Users
// NFSHomeDirectory`: the first two blank-separated fields, so a home with a
// space in it is cut at the space, the home without its empty segments
// (withoutEmptySegments) as a passwd home is.
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
			homes = append(homes, Home{fields[0], withoutEmptySegments(fields[1])})
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
	for _, pattern := range imageHomeGlobs() {
		for _, match := range guard.expand(root, pattern) {
			if isDir, err := guard.examine(match); isDir || err != nil {
				homes = append(homes, Home{match[strings.LastIndexByte(match, '/')+1:], match})
			}
		}
	}
	return homes
}

// seenHome is a home filterHomes has seen: its index among the candidates,
// and among the report's Homes (-1 for a skipped one).
type seenHome struct{ candidate, home int }

// filterHomes returns the report's homes, skipped candidates, found users
// and merges: it drops the homes rule skips, candidates that are not
// directories or do not exist, users the -u list did not name (under rule's
// comparison), and every home already seen. Two paths are one home when
// their bytes are equal, or differ only in ASCII case and follow.SameEntry
// gives them one identity; nothing is listed. written holds each candidate's
// path as written, before reaching; candidates from firstListed on came from
// a home parent's listing, which respells a kept home they repeat (repeat)
// even when -u leaves their user out, with no merge line then.
func filterHomes(candidates []Home, written []string, firstListed int, filtered bool,
	names []string, guard *rootGuard, rule homeRule,
) (report Report) {
	seen := syspath.NewEntriesIgnoringCase[seenHome](follow.SameEntry(guard.check))
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
			if first, _, repeated := seen.Find(home.Path); repeated && isDir && first.home >= 0 &&
				i >= firstListed {
				report.respell(first.home, home.Path)
			}
			continue
		}
		if first, _, repeated := seen.Find(home.Path); repeated {
			// Only a directory is a home Find returns and walks once.
			if isDir && first.home >= 0 {
				report.repeat(candidates, written, first, i, i >= firstListed)
			}
			continue
		}
		if isDir {
			seen.Put(home.Path, seenHome{i, len(report.Homes)})
			report.Homes = append(report.Homes, home)
		} else {
			seen.Put(home.Path, seenHome{i, -1})
			report.Skipped = append(report.Skipped, Skipped{User: home.User, Path: home.Path,
				Reason: err.Error(), Denied: parentLists(home.Path)})
		}
	}
	return report
}

// repeat records candidates[i], a repeat of the home first kept. The kept
// home takes the repeat's spelling when a home parent's listing gave it
// (listed), since that is the disk's spelling; a passwd or dscl repeat
// leaves the first spelling. The repeat is a merge, reported under the kept
// home's user, when its written path differs from the kept home's.
func (report *Report) repeat(candidates []Home, written []string, first seenHome, i int,
	listed bool,
) {
	if listed {
		report.respell(first.home, candidates[i].Path)
	}
	kept := &report.Homes[first.home]
	if written[first.candidate] != written[i] {
		report.Merged = append(report.Merged, Merge{Path: kept.Path,
			Kept: candidates[first.candidate].User, Dropped: candidates[i].User})
	}
}

// respell gives the kept home at index home the spelling path, the one a home
// parent's listing gave, and its earlier merge lines with it.
func (report *Report) respell(home int, path string) {
	kept := &report.Homes[home]
	if kept.Path == path {
		return
	}
	for j := range report.Merged {
		if report.Merged[j].Path == kept.Path {
			report.Merged[j].Path = path
		}
	}
	kept.Path = path
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
