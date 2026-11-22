package input

import (
	"strings"
	"unicode/utf16"
)

// maxWindowsName is the longest file or directory name the Windows file
// systems store, in UTF-16 code units.
const maxWindowsName = 255

// windowsNameAllowed reports whether Windows stores name, one path
// component, as a file or directory of that name that its own programs can
// open again: no character of <>:"/\|?* and no control character, no space
// or dot at the end, at most maxWindowsName UTF-16 code units, and not a
// device name (CON, PRN, AUX, NUL, COM0 to COM9, LPT0 to LPT9, and COM and
// LPT with a superscript digit), with or without an extension or trailing
// spaces before it, in any ASCII case. Microsoft's "Naming Files, Paths,
// and Namespaces" lists these rules.
func windowsNameAllowed(name string) bool {
	if name == "" || strings.ContainsAny(name, `<>:"/\|?*`) || strings.HasSuffix(name, ".") ||
		strings.HasSuffix(name, " ") {
		return false
	}
	units := 0
	for _, char := range name {
		if char < 0x20 {
			return false
		}
		units += utf16.RuneLen(char)
	}
	return units <= maxWindowsName && !windowsDeviceName(name)
}

// windowsDeviceName reports whether name, up to its first dot and less the
// spaces before that dot, is a Windows device name in any ASCII case.
func windowsDeviceName(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	stem = strings.ToUpper(strings.TrimRight(stem, " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(stem) < 4 || (stem[:3] != "COM" && stem[:3] != "LPT") {
		return false
	}
	switch stem[3:] {
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
		return true
	}
	return false
}
