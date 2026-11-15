package fsmeta

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// OnDiskName returns the spelling directory holds for name, which
// may differ from name in case, and true, as the PowerShell collector's
// Get-OnDiskName respells a literal catalog segment. It returns name and
// false for . and .., when name holds a character the directory search reads
// as a wildcard, when nothing or more than one entry matches, or when the
// match is through the entry's short 8.3 name (see onDiskEntry).
func OnDiskName(directory, name string) (string, bool) {
	entry := onDiskEntry(directory, name)
	return entry.spelling, entry.respelled
}

// OnDiskPath returns path with each existing component below base spelled
// as its directory holds it, so two spellings of one directory become the
// same bytes without the binary judging case itself: the file system's own
// name comparison decides. It stops at a link: a component that is a
// symlink, junction or other reparse point is respelled from its own
// directory entry and nothing behind it is looked at. base ("" for the
// path's drive or share) and everything above it are never looked at and
// stay as written; a path not under base comes back unchanged. Respelling
// also stops at a missing component and at . or ..; a component holding a
// wildcard or matching only as a short 8.3 name stays as written.
func OnDiskPath(base, path string) string {
	if base == "" {
		base = volumeOf(path)
	}
	rest, found := syspath.CutPrefix(path, base+"/")
	if base == "" || !found || rest == "" {
		return path
	}
	prefix := path[:len(path)-len(rest)-1]
	directory := prefix
	components := strings.Split(rest, "/")
	for i, component := range components {
		entry := onDiskEntry(directory, component)
		if !entry.exists {
			break
		}
		components[i] = entry.spelling
		if entry.attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			break
		}
		directory += "/" + entry.spelling
	}
	return prefix + "/" + strings.Join(components, "/")
}

// diskEntry is what onDiskEntry found for a name.
type diskEntry struct {
	spelling   string
	exists     bool
	respelled  bool
	attributes uint32
}

// onDiskEntry looks name up in directory with FindFirstFile, which reads the
// directory's entry and follows nothing. exists is true when one entry
// matches, with its attributes; spelling is the entry's name when respelled
// is true, else name as given: for . or .., a wildcard character in name,
// several matches, or a short 8.3 name typed for a different long name (the
// typed name equals the short name ignoring ASCII case, and the long name
// does not). A short name typed with a non-ASCII letter in another case
// is respelled to the long name, which names the same entry.
func onDiskEntry(directory, name string) diskEntry {
	given := diskEntry{spelling: name}
	// Extended resolves . and .. by name, so the search would name another
	// directory's entry.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `*?<>"`) {
		return given
	}
	pattern, err := windows.UTF16PtrFromString(syspath.Extended(directory + "/" + name))
	if err != nil {
		return given
	}
	var data windows.Win32finddata
	handle, err := windows.FindFirstFile(pattern, &data)
	if err != nil {
		return given
	}
	defer windows.FindClose(handle)
	var another windows.Win32finddata
	if windows.FindNextFile(handle, &another) == nil {
		return given
	}
	given.exists, given.attributes = true, data.FileAttributes
	found := windows.UTF16ToString(data.FileName[:])
	short := windows.UTF16ToString(data.AlternateFileName[:])
	if !syspath.Equal(found, name) && short != "" && syspath.Equal(short, name) {
		return given
	}
	given.spelling, given.respelled = found, true
	return given
}

// volumeOf returns the drive (C:) or share (//server/share) a canonical path
// starts with, or "" when it starts with neither.
func volumeOf(path string) string {
	if len(path) >= 2 && path[1] == ':' && (len(path) == 2 || path[2] == '/') {
		return path[:2]
	}
	if !strings.HasPrefix(path, "//") {
		return ""
	}
	server, rest, found := strings.Cut(path[2:], "/")
	if !found || server == "" {
		return ""
	}
	share, _, _ := strings.Cut(rest, "/")
	if share == "" {
		return ""
	}
	return "//" + server + "/" + share
}
