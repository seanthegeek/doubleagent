//go:build unix

package users

import (
	"bytes"
	"context"
	"os"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/glob"
)

// homeGlobs returns the usual home parents, in the order the sh collector
// tries them, each relative to the root.
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

// Find returns the homes to collect, in enumeration order. In live mode
// the passwd text comes from `getent passwd` and macOS adds `dscl`; a program
// that fails is treated as absent. Otherwise passwd is root/etc/passwd. The
// usual home parents follow. usersFilter is the -u list, compared
// case-sensitively; a home listed twice keeps its first user; only existing
// directories are returned. root is "" or "/" for the live system. Once ctx
// is cancelled no program is run, as if none were installed; the caller
// checks ctx before using the report.
func Find(ctx context.Context, live bool, root, usersFilter string, run Runner) Report {
	root = strings.TrimSuffix(root, "/")
	candidates := passwdHomes(ctx, live, root, run)
	if live {
		candidates = append(candidates, dsclHomes(ctx, run)...)
	}
	candidates = append(candidates, globHomes(root)...)
	kept, found := filterHomes(candidates, root, usersFilter)
	var homes []Home
	for _, home := range kept {
		if isDirectory(home.Path) {
			homes = append(homes, home)
		}
	}
	return Report{Homes: homes, Found: found,
		Unmatched: unmatchedUsers(userNames(usersFilter, sameName), found, sameName)}
}

// sameName reports whether a -u name names user: byte for byte, since Unix
// account names are case-sensitive.
func sameName(name, user string) bool { return name == user }

// passwdHomes returns the user and root-prefixed home of every passwd line
// with at least six fields and a non-empty sixth. Live, the text is
// getent's when getent runs, else the file's, as the sh collector does.
func passwdHomes(ctx context.Context, live bool, root string, run Runner) []Home {
	var text []byte
	ran := false
	if live && run != nil && ctx.Err() == nil {
		output, err := run(ctx, "getent", "passwd")
		text, ran = output, err == nil
	}
	if !ran {
		// test -f follows a link; a directory or missing file gives nothing.
		text, _ = os.ReadFile(root + "/etc/passwd")
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
// space in it is cut at the space, as the sh collector's awk cuts it.
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

// isBlank is awk's default field separator under LC_ALL=C: space, tab and newline.
func isBlank(char rune) bool { return char == ' ' || char == '\t' || char == '\n' }

// globHomes returns, for each home parent in order, the entries that exist
// and are directories, each named by its last path segment. A match holding a
// newline is read as its lines, each tried as a home, since the sh collector
// reads the expansion line by line.
func globHomes(root string) []Home {
	var homes []Home
	for _, pattern := range homeGlobs() {
		for _, match := range glob.Expand(root, pattern) {
			for _, line := range strings.Split(match, "\n") {
				if isDirectory(line) {
					homes = append(homes, Home{line[strings.LastIndexByte(line, '/')+1:], line})
				}
			}
		}
	}
	return homes
}

// filterHomes drops the system and shared profile homes, the users the -u
// list did not name, and every home already seen. A home under root is
// compared with the system list without the root; the root counts as a prefix
// of the text, so "/tmp/x" also matches "/tmp/xyz/home". found lists the
// users of the candidates that pass every test but -u's and are directories.
func filterHomes(candidates []Home, root, usersFilter string) (kept []Home, found []string) {
	wanted, filtered := wantedUsers(usersFilter)
	seen := map[string]struct{}{}
	for _, home := range candidates {
		relative := home.Path
		if root != "" && strings.HasPrefix(home.Path, root) {
			relative = home.Path[len(root):]
		}
		if isSystemHome(relative) {
			continue
		}
		if isDirectory(home.Path) {
			found = addFound(found, home.User)
		}
		if _, named := wanted[home.User]; filtered && !named {
			continue
		}
		if _, repeated := seen[home.Path]; repeated {
			continue
		}
		seen[home.Path] = struct{}{}
		kept = append(kept, home)
	}
	return kept, found
}

// wantedUsers splits the -u list at commas. filtered is true for any
// non-empty list, even one that names nobody (","), which then keeps no home.
func wantedUsers(usersFilter string) (wanted map[string]struct{}, filtered bool) {
	wanted = map[string]struct{}{}
	if usersFilter == "" {
		return wanted, false
	}
	for _, name := range strings.Split(usersFilter, ",") {
		if name != "" {
			wanted[name] = struct{}{}
		}
	}
	return wanted, true
}

// isDirectory is test -d: true for a directory or a link to one.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
