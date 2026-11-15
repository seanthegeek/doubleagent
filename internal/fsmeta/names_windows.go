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
// as a wildcard, when nothing or more than one entry matches, or when name
// matches only through the entry's short 8.3 name: a catalog literal names
// a long name, never a short one.
func OnDiskName(directory, name string) (string, bool) {
	entry := onDiskEntry(directory, name)
	if !entry.exists || entry.shortOnly {
		return name, false
	}
	return entry.name, true
}

// OnDiskPath returns path with each existing component below base spelled
// as its directory holds it, a short 8.3 name (JOHNSM~1) as the long name,
// so every spelling of one directory becomes the same bytes; the file
// system's own name comparison decides, not the binary. It stops at a link:
// a symlink, junction or other reparse point is respelled from its own
// directory entry and nothing behind it is looked at. base ("" for the
// path's drive or share) and everything above it are never looked at and
// stay as written; a path not under base comes back unchanged. Respelling
// also stops at a missing component, at . or .., and at a wildcard.
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
		components[i] = entry.name
		if entry.attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			break
		}
		directory += "/" + entry.name
	}
	return prefix + "/" + strings.Join(components, "/")
}

// diskEntry is what onDiskEntry found for a name: whether exactly one entry
// matches, its long name and attributes, and whether the name matched only
// its short 8.3 name.
type diskEntry struct {
	name       string
	exists     bool
	shortOnly  bool
	attributes uint32
}

// onDiskEntry looks name up in directory with FindFirstFile, which reads the
// directory's entry and follows nothing. Nothing exists for . or .., a
// wildcard character in name, or several matches. shortOnly is set when the
// typed name equals the entry's short name ignoring ASCII case and its long
// name does not; a short name typed with a non-ASCII letter in another case
// is not told apart, and either way the match is the same entry.
func onDiskEntry(directory, name string) diskEntry {
	// Extended resolves . and .. by name, so the search would name another
	// directory's entry.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `*?<>"`) {
		return diskEntry{}
	}
	pattern, err := windows.UTF16PtrFromString(syspath.Extended(directory + "/" + name))
	if err != nil {
		return diskEntry{}
	}
	var data windows.Win32finddata
	handle, err := windows.FindFirstFile(pattern, &data)
	if err != nil {
		return diskEntry{}
	}
	defer windows.FindClose(handle)
	var another windows.Win32finddata
	if windows.FindNextFile(handle, &another) == nil {
		return diskEntry{}
	}
	found := windows.UTF16ToString(data.FileName[:])
	short := windows.UTF16ToString(data.AlternateFileName[:])
	return diskEntry{name: found, exists: true, attributes: data.FileAttributes,
		shortOnly: !syspath.Equal(found, name) && short != "" && syspath.Equal(short, name)}
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
