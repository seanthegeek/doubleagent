package projects

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// pathRules says how localProjectPath turns a value a discovery source holds
// into a project path. The zero value is the POSIX rule set; the
// platform files give the rules of the host the binary runs on.
type pathRules struct {
	// driveLetters reads values with blanks trimmed,
	// `\\` and `\/` undone before any test, a drive path (`C:\x`, `c:/x`)
	// accepted with its letter upper-cased, and trailing separators trimmed.
	driveLetters bool
	// decodeEscapes takes a `file://` value as a URI: the scheme dropped,
	// every %XX escape decoded and the slash before a drive (`/C:/x`) dropped.
	decodeEscapes bool
	// dropPOSIXLive drops a value starting with / on a live run, since it
	// names no place on a Windows host.
	dropPOSIXLive bool
	// posixOthers keeps the POSIX rule, on the value as written, for every
	// value driveLetters does not read as a drive path. Under it a value is
	// a drive value only as a drive path as written or a drive `file://`
	// URL: a raw `/C:/x` names a directory `C:` on Linux and macOS.
	posixOthers bool
	// driveJSON widens the JSON key and string readers to drive paths:
	// a key starting with a letter and a colon, a string starting with a
	// letter, a colon and an escaped backslash (`"C:\\x"`).
	driveJSON bool
	// spell, when set, spells a drive path below root as the disk holds it
	// in place of fsmeta.OnDiskPath.
	spell func(root, path string) string
}

// asciiBlanks are the ASCII white-space bytes trimmed from a value; a value
// is a byte string, so no UTF-8 is decoded to find other blanks.
const asciiBlanks = " \t\n\v\f\r"

// hexDigit returns the value of the hexadecimal digit at position in text,
// and false when position is past the end or holds another byte.
func hexDigit(text string, position int) (byte, bool) {
	if position >= len(text) {
		return 0, false
	}
	switch digit := text[position]; {
	case '0' <= digit && digit <= '9':
		return digit - '0', true
	case 'a' <= digit && digit <= 'f':
		return digit - 'a' + 10, true
	case 'A' <= digit && digit <= 'F':
		return digit - 'A' + 10, true
	}
	return 0, false
}

// percentDecode replaces every %XX escape, XX two hexadecimal digits of
// either case, by the byte it encodes, and keeps any other % as it is, so
// a malformed escape never fails the value.
func percentDecode(text string) string {
	if !strings.Contains(text, "%") {
		return text
	}
	decoded := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		high, highOK := hexDigit(text, i+1)
		low, lowOK := hexDigit(text, i+2)
		if text[i] == '%' && highOK && lowOK {
			decoded = append(decoded, high<<4|low)
			i += 2
			continue
		}
		decoded = append(decoded, text[i])
	}
	return string(decoded)
}

// fileURIPath returns the path of a file:// URI, percent-decoded whole,
// without the slash a drive URI puts before its letter; any other value
// comes back unchanged.
func fileURIPath(value string) string {
	if !strings.HasPrefix(value, "file://") {
		return value
	}
	value = percentDecode(value[len("file://"):])
	if len(value) >= 3 && value[0] == '/' && evidencepath.IsASCIILetter(value[1]) && value[2] == ':' {
		value = value[1:]
	}
	return value
}

// driveFromURIPath returns the drive path a decoded URI path names when a
// reader has already cut its file:// scheme: /C:/x and /c:/x become C:/x and
// c:/x. Any other value, including a head not followed by a separator,
// comes back unchanged and stays a POSIX path.
func driveFromURIPath(value string) string {
	if strings.HasPrefix(value, "/") && evidencepath.IsDrivePath(value[1:]) {
		return value[1:]
	}
	return value
}

// driveValue returns value as the drive-letter rules read it, blanks
// trimmed, a file:// URI decoded and `\\` and `\/` undone, and whether
// that is a drive path; under any other rules, value and false. A raw
// `/C:/x`, a URL path whose reader cut the scheme, is a drive path only
// without posixOthers, as on Windows, where it names no other place.
func (rules pathRules) driveValue(value string) (string, bool) {
	if !rules.driveLetters {
		return value, false
	}
	value = strings.Trim(value, asciiBlanks)
	if rules.decodeEscapes {
		value = fileURIPath(value)
	}
	value = strings.ReplaceAll(strings.ReplaceAll(value, `\\`, `\`), `\/`, "/")
	if !rules.posixOthers {
		value = driveFromURIPath(value)
	}
	return value, evidencepath.IsDrivePath(value)
}

// localProjectPath returns the canonical project path a discovered value
// names, under root ("" live) in image mode, and false when the value names
// no absolute path the host or image can hold. Under the POSIX rules a value
// must start with /, then has every `\/` undone and root put before it.
// Under the drive-letter rules a drive path is put below root in an image,
// and its existing components below root are spelled as the disk holds them
// up to the first link (rules.spell, else fsmeta.OnDiskPath; a missing one
// stays).
func localProjectPath(value, root string, rules pathRules) (string, bool) {
	text, isDrive := rules.driveValue(value)
	switch {
	case !rules.driveLetters || !isDrive && rules.posixOthers:
		if !strings.HasPrefix(value, "/") {
			return "", false
		}
		return root + strings.ReplaceAll(value, `\/`, "/"), true
	case isDrive:
		value = string(text[0]&^0x20) + text[1:]
		if root != "" {
			value = root + value[2:]
		}
	case strings.HasPrefix(text, "/"):
		if root == "" && rules.dropPOSIXLive {
			return "", false
		}
		value = root + text
	default:
		return "", false
	}
	// A value is a file's text, in whatever case its writer used; respelled
	// as the disk holds it, below root only and up to the first link, it
	// compares byte for byte with listed paths. On Windows its `.` and `..`
	// are first removed by name as Win32 removes them (syspath.Opened), so
	// the one string the run checks is the one it opens, stages and logs.
	value = syspath.Opened(strings.TrimRight(strings.ReplaceAll(value, `\`, "/"), "/"))
	if rules.spell != nil {
		value = rules.spell(root, value)
	} else {
		value = fsmeta.OnDiskPath(root, value)
	}
	return value, value != ""
}
