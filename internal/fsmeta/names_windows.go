package fsmeta

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// OnDiskName returns the spelling directory holds for name, which
// may differ from name in case, and true, as the PowerShell collector's
// Get-OnDiskName respells a literal catalog segment. It returns name and
// false when name holds a character the directory search reads as a
// wildcard, when nothing or more than one entry matches, or when the match
// differs from name by more than case (a short 8.3 name matched).
func OnDiskName(directory, name string) (string, bool) {
	if strings.ContainsAny(name, `*?<>"`) {
		return name, false
	}
	pattern, err := windows.UTF16PtrFromString(syspath.Extended(directory + "/" + name))
	if err != nil {
		return name, false
	}
	var data windows.Win32finddata
	handle, err := windows.FindFirstFile(pattern, &data)
	if err != nil {
		return name, false
	}
	defer windows.FindClose(handle)
	found := windows.UTF16ToString(data.FileName[:])
	var another windows.Win32finddata
	if windows.FindNextFile(handle, &another) == nil || !syspath.Equal(found, name) {
		return name, false
	}
	return found, true
}
