// Package users finds the home directories to collect, reproducing the sh
// collector's enumerate_users, filter_users and get_user_homes.
package users

import (
	"bytes"
	"os"
	"sort"
	"strings"
)

// Home is a user and the home directory to collect for them. Path includes the
// image root, as the sh collector builds it.
type Home struct {
	User string
	Path string
}

// Runner runs a program and returns its standard output. hostcmd's Runner
// provides it, as a method value, so this package does not import hostcmd.
type Runner func(name string, args ...string) ([]byte, error)

// homeGlobs are the usual home parents, in the order the sh collector tries
// them, each relative to the root.
var homeGlobs = [...]string{"home/*", "Users/*", "root", "var/root", "usr/home/*", "export/home/*"}

// systemHomes are the homes, relative to the root, of system and shared
// profiles that are never collected.
var systemHomes = map[string]struct{}{
	"": {}, "/": {}, "/bin": {}, "/sbin": {}, "/usr": {}, "/usr/bin": {}, "/usr/sbin": {},
	"/dev": {}, "/dev/null": {}, "/proc": {}, "/sys": {}, "/nonexistent": {},
	"/var/empty": {}, "/Users/Shared": {}, "/Users/Public": {}, "/Users/Default": {},
	"/Users/All Users": {}, "/Users/Default User": {},
}

// Enumerate returns the homes to collect, in enumeration order. In live mode
// the passwd text comes from `getent passwd` and macOS adds `dscl`; a program
// that fails is treated as absent. Otherwise passwd is root/etc/passwd. The
// usual home parents follow. usersFilter is the -u list, compared
// case-sensitively; a home listed twice keeps its first user; only existing
// directories are returned. root is "" or "/" for the live system.
func Enumerate(live bool, root, usersFilter string, run Runner) []Home {
	root = strings.TrimSuffix(root, "/")
	candidates := passwdHomes(live, root, run)
	if live {
		candidates = append(candidates, dsclHomes(run)...)
	}
	candidates = append(candidates, globHomes(root)...)
	var homes []Home
	for _, home := range filterHomes(candidates, root, usersFilter) {
		if isDirectory(home.Path) {
			homes = append(homes, home)
		}
	}
	return homes
}

// passwdHomes returns the user and root-prefixed home of every passwd line
// with at least six fields and a non-empty sixth. Live, the text is
// getent's when getent runs, else the file's, as the sh collector does.
func passwdHomes(live bool, root string, run Runner) []Home {
	var text []byte
	ran := false
	if live && run != nil {
		output, err := run("getent", "passwd")
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
func dsclHomes(run Runner) []Home {
	if run == nil {
		return nil
	}
	output, err := run("dscl", ".", "-list", "/Users", "NFSHomeDirectory")
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
// and are directories, each named by its last path segment.
func globHomes(root string) []Home {
	var homes []Home
	for _, pattern := range homeGlobs {
		for _, match := range expandHomeGlob(root, pattern) {
			if isDirectory(match) {
				homes = append(homes, Home{match[strings.LastIndexByte(match, '/')+1:], match})
			}
		}
	}
	return homes
}

// filterHomes drops the system and shared profile homes, the users the -u
// list did not name, and every home already seen. A home under root is
// compared with the system list without the root; the root counts as a prefix
// of the text, so "/tmp/x" also matches "/tmp/xyz/home".
func filterHomes(candidates []Home, root, usersFilter string) []Home {
	wanted, filtered := wantedUsers(usersFilter)
	seen := map[string]struct{}{}
	var kept []Home
	for _, home := range candidates {
		relative := home.Path
		if root != "" && strings.HasPrefix(home.Path, root) {
			relative = home.Path[len(root):]
		}
		if _, system := systemHomes[relative]; system {
			continue
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
	return kept
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

// expandHomeGlob expands one of the home globs inside root as a shell does,
// and is the one place this package expands a pattern: literal segments are
// kept untested, a trailing * lists the directory (open, ReadDir(-1)) and
// keeps the names that do not start with a dot in bytewise order, and a result
// stays when it exists or is a symlink. Every home glob is a literal path
// optionally ending in /*.
func expandHomeGlob(root, pattern string) []string {
	candidates := []string{root + "/" + pattern}
	if parent, found := strings.CutSuffix(pattern, "/*"); found {
		candidates = candidates[:0]
		directory := root + "/" + parent
		handle, err := os.Open(directory)
		if err != nil {
			return nil
		}
		defer handle.Close()
		entries, _ := handle.ReadDir(-1)
		for _, entry := range entries {
			if name := entry.Name(); !strings.HasPrefix(name, ".") {
				candidates = append(candidates, directory+"/"+name)
			}
		}
		sort.Strings(candidates)
	}
	var matches []string
	for _, candidate := range candidates {
		if _, err := os.Lstat(candidate); err == nil {
			matches = append(matches, candidate)
		}
	}
	return matches
}
