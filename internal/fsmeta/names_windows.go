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
// match is the entry's short 8.3 name rather than a spelling of its name.
func OnDiskName(directory, name string) (string, bool) {
	spelling, _, respelled := onDiskEntry(directory, name)
	return spelling, respelled
}

// OnDiskPath returns path, canonical and absolute, with each existing
// component spelled as its directory holds it, so two spellings of one
// directory become the same bytes without the binary judging case itself:
// the file system's own name comparison decides. The drive or share stays as
// written, and so does a component that holds a search wildcard or matches
// only as a short 8.3 name. Respelling stops at the first component that
// does not exist and at the first . or .. (behind a link, .. does not go
// back to the directory the text names), leaving the rest as written; a
// relative path comes back unchanged. Naming a component goes through the
// links before it, as opening the path would; the last is never followed.
func OnDiskPath(path string) string {
	volume := volumeOf(path)
	if volume == "" || len(path) <= len(volume)+1 {
		return path
	}
	components := strings.Split(path[len(volume)+1:], "/")
	directory := volume
	for i, component := range components {
		if component == "" || component == "." || component == ".." {
			break
		}
		spelling, exists, _ := onDiskEntry(directory, component)
		if !exists {
			break
		}
		components[i] = spelling
		directory += "/" + spelling
	}
	return volume + "/" + strings.Join(components, "/")
}

// onDiskEntry looks name up in directory with FindFirstFile. exists is true
// when one entry matches; spelling is that entry's name when respelled is
// true, else name as given: for . or .., a wildcard character in name,
// several matches, or a match through the entry's short 8.3 name.
func onDiskEntry(directory, name string) (spelling string, exists, respelled bool) {
	// Extended resolves . and .. by name, so the search would name another
	// directory's entry.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `*?<>"`) {
		return name, false, false
	}
	pattern, err := windows.UTF16PtrFromString(syspath.Extended(directory + "/" + name))
	if err != nil {
		return name, false, false
	}
	var data windows.Win32finddata
	handle, err := windows.FindFirstFile(pattern, &data)
	if err != nil {
		return name, false, false
	}
	defer windows.FindClose(handle)
	var another windows.Win32finddata
	if windows.FindNextFile(handle, &another) == nil {
		return name, false, false
	}
	found := windows.UTF16ToString(data.FileName[:])
	short := windows.UTF16ToString(data.AlternateFileName[:])
	if !syspath.Equal(found, name) && short != "" && syspath.Equal(short, name) {
		return name, true, false
	}
	return found, true, true
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
