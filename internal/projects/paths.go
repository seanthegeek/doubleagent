package projects

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// pathRules says how localProjectPath turns a value a discovery source holds
// into a project path. The zero value is the sh collector's rule; the
// platform files give the rules of the host the binary runs on.
type pathRules struct {
	// driveLetters reads values as ConvertTo-LocalPath does: blanks trimmed,
	// `\\` and `\/` undone before any test, a drive path (`C:\x`, `c:/x`)
	// accepted with its letter upper-cased, and trailing separators trimmed.
	driveLetters bool
	// decodeEscapes takes a `file://` value as a URI: the scheme dropped,
	// every %XX escape decoded and the slash before a drive (`/C:/x`) dropped.
	decodeEscapes bool
	// dropPOSIXLive drops a value starting with / on a live run, since it
	// names no place on a Windows host.
	dropPOSIXLive bool
	// driveJSON widens the JSON key and string readers to drive paths, as
	// the PowerShell collector's Get-JsonKeys and Get-PathStrings read them:
	// a key starting with a letter and a colon, a string starting with a
	// letter, a colon and an escaped backslash (`"C:\\x"`).
	driveJSON bool
}

// asciiBlanks are the bytes .NET's String.Trim removes that a path can hold
// as single bytes; a value is a byte string, so no UTF-8 is decoded to find
// the multi-byte ones.
const asciiBlanks = " \t\n\v\f\r"

// localProjectPath returns the canonical project path a discovered value
// names, under root ("" live) in image mode, and false when the value names
// no absolute path the host or image can hold. Under the sh rules a value
// must start with /, then has every `\/` undone and root put before it.
// Under the drive-letter rules the existing components of the result below
// root are spelled as the disk holds them, up to the first link
// (fsmeta.OnDiskPath; a missing one stays).
func localProjectPath(value, root string, rules pathRules) (string, bool) {
	if !rules.driveLetters {
		if !strings.HasPrefix(value, "/") {
			return "", false
		}
		return root + strings.ReplaceAll(value, `\/`, "/"), true
	}
	value = strings.Trim(value, asciiBlanks)
	if rules.decodeEscapes {
		value = fileURIPath(value)
	}
	value = strings.ReplaceAll(strings.ReplaceAll(value, `\\`, `\`), `\/`, "/")
	value = driveFromURIPath(value)
	switch {
	case evidencepath.IsDrivePath(value):
		value = string(upperASCII(value[0])) + value[1:]
		if root != "" {
			value = root + value[2:]
		}
	case strings.HasPrefix(value, "/"):
		if root == "" && rules.dropPOSIXLive {
			return "", false
		}
		value = root + value
	default:
		return "", false
	}
	// A value is a file's text, in whatever case its writer used; respelled
	// as the disk holds it, below root only and up to the first link, it
	// compares byte for byte with listed paths. On Windows its `.` and `..`
	// are first removed by name as Win32 removes them (syspath.Opened), so
	// the one string the run checks is the one it opens, stages and logs.
	value = syspath.Opened(strings.TrimRight(strings.ReplaceAll(value, `\`, "/"), "/"))
	value = fsmeta.OnDiskPath(root, value)
	return value, value != ""
}

// fileURIPath returns the path of a file:// URI, percent-decoded whole as
// Uri.UnescapeDataString decodes it, without the slash a drive URI puts
// before its letter; any other value comes back unchanged.
func fileURIPath(value string) string {
	if !strings.HasPrefix(value, "file://") {
		return value
	}
	value = percentDecode(value[len("file://"):])
	if len(value) >= 3 && value[0] == '/' && isLetter(value[1]) && value[2] == ':' {
		value = value[1:]
	}
	return value
}

// driveFromURIPath returns the drive path a URI path names when a reader
// has already cut its file:// scheme: /C:/x and /c%3A/x (either case of the
// hex) become C:/x and c:/x. Any other value, including a head not followed
// by a separator, comes back unchanged and stays a POSIX path.
func driveFromURIPath(value string) string {
	if len(value) < 3 || value[0] != '/' || !isLetter(value[1]) {
		return value
	}
	rest := value[2:]
	switch {
	case strings.HasPrefix(rest, ":"):
		rest = rest[1:]
	case len(rest) >= 3 && rest[0] == '%' && rest[1] == '3' && (rest[2] == 'A' || rest[2] == 'a'):
		rest = rest[3:]
	default:
		return value
	}
	if drive := value[1:2] + ":" + rest; evidencepath.IsDrivePath(drive) {
		return drive
	}
	return value
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

// upperASCII returns char upper-cased when it is an ASCII small letter.
func upperASCII(char byte) byte {
	if 'a' <= char && char <= 'z' {
		return char - 'a' + 'A'
	}
	return char
}
