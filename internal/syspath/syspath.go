package syspath

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// Equal reports whether two paths are the same key: byte equality on Unix,
// equality ignoring ASCII case on Windows (see Fold for why ASCII only). It
// cannot tell two entries of a case-sensitive directory apart; SameAs can.
func Equal(first, second string) bool {
	return Fold(first) == Fold(second)
}

// HasPrefix reports whether path starts with prefix, compared as Equal does.
func HasPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && Fold(path[:len(prefix)]) == Fold(prefix)
}

// CutPrefix returns path without prefix and true when path starts with it,
// compared as Equal does, else path unchanged and false. The rest keeps the
// path's own spelling.
func CutPrefix(path, prefix string) (rest string, found bool) {
	if !HasPrefix(path, prefix) {
		return path, false
	}
	return path[len(prefix):], true
}

// fromWindows turns a native Windows path into the canonical form: the
// extended prefix dropped (\\?\UNC\ back to //), backslashes made forward and
// one trailing separator trimmed unless the path is a lone /. The UNC prefix
// is matched ignoring case, the plain one exactly.
func fromWindows(path string) string {
	switch {
	case hasFoldedPrefix(path, `\\?\UNC\`):
		path = `\\` + path[len(`\\?\UNC\`):]
	case strings.HasPrefix(path, `\\?\`):
		path = path[len(`\\?\`):]
	}
	path = strings.ReplaceAll(path, `\`, "/")
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}

// toWindows turns a canonical path into the native Windows form: forward
// slashes made backslashes, and a bare drive gets its root separator back,
// since C: alone means the current directory of drive C.
func toWindows(path string) string {
	path = strings.ReplaceAll(path, "/", `\`)
	if len(path) == 2 && isDriveLetter(path[0]) && path[1] == ':' {
		path += `\`
	}
	return path
}

// extendedWindows returns the \\?\ form of an absolute Windows path, in
// either slash form, with . and .. segments resolved, since Win32 resolves
// neither behind the prefix. A relative or already prefixed path comes back
// unchanged. The prefix is added whatever the length.
func extendedWindows(path string) string {
	prefix, body, keep, ok := extendedParts(path)
	if !ok {
		return path
	}
	return prefix + resolveSegments(body, keep)
}

// openedWindows returns the path Windows opens for path, in canonical form:
// an absolute path has its empty, . and .. segments removed by name, by the
// same resolveSegments that Extended uses, since Win32 removes them before
// the file system sees the path; a relative path comes back unchanged.
func openedWindows(path string) string {
	extended := extendedWindows(path)
	if extended == path {
		return path
	}
	return fromWindows(extended)
}

// extendedParts splits path, its slashes made backslashes, into the extended
// prefix to put before it, the body after that prefix and how many leading
// segments of the body are never popped (the drive, or server and share).
// ok is false for a relative path and for one already starting with \\?\ or
// the \\.\ device prefix. A bare drive is its root, as in the canonical form.
func extendedParts(path string) (prefix, body string, keep int, ok bool) {
	backslashed := strings.ReplaceAll(path, "/", `\`)
	switch {
	case len(backslashed) == 2 && isDriveLetter(backslashed[0]) && backslashed[1] == ':':
		return `\\?\`, backslashed + `\`, 1, true
	case len(backslashed) >= 3 && isDriveLetter(backslashed[0]) && backslashed[1] == ':' &&
		backslashed[2] == '\\':
		return `\\?\`, backslashed, 1, true
	case len(backslashed) > 2 && strings.HasPrefix(backslashed, `\\`) &&
		backslashed[2] != '?' && backslashed[2] != '.':
		return `\\?\UNC\`, backslashed[2:], 2, true
	}
	return "", "", 0, false
}

// resolveSegments drops the empty and . segments of a backslashed body and
// pops one segment per .., never below its first keep segments; a body left
// with only those ends in a backslash, since \\?\C: alone names no directory.
func resolveSegments(body string, keep int) string {
	kept := make([]string, 0, strings.Count(body, `\`)+1)
	for _, segment := range strings.Split(body, `\`) {
		switch {
		case len(kept) < keep:
			kept = append(kept, segment)
		case segment == "" || segment == ".":
		case segment == "..":
			if len(kept) > keep {
				kept = kept[:len(kept)-1]
			}
		default:
			kept = append(kept, segment)
		}
	}
	resolved := strings.Join(kept, `\`)
	if len(kept) <= keep {
		resolved += `\`
	}
	return resolved
}

// foldASCII returns text with the ASCII capitals lowered and every other byte
// kept, so the result has the same length and is a comparison key, never
// output.
func foldASCII(text string) string {
	folded := []byte(text)
	for i, character := range folded {
		if 'A' <= character && character <= 'Z' {
			folded[i] = character + 'a' - 'A'
		}
	}
	return string(folded)
}

// hasFoldedPrefix reports whether text starts with prefix ignoring ASCII
// case, on every platform.
func hasFoldedPrefix(text, prefix string) bool {
	return len(text) >= len(prefix) && foldASCII(text[:len(prefix)]) == foldASCII(prefix)
}

// archiveRelWindows returns the archive form of a canonical path below fs/:
// a drive path C:/a becomes /C/a with the drive upper-cased, a UNC path
// //server/share/a becomes /server/share/a, and anything else (the rest of a
// path below an image root) is unchanged.
func archiveRelWindows(path string) string {
	switch {
	case len(path) >= 2 && isDriveLetter(path[0]) && path[1] == ':' &&
		(len(path) == 2 || path[2] == '/'):
		return "/" + strings.ToUpper(path[:1]) + path[2:]
	case strings.HasPrefix(path, "//"):
		return path[1:]
	}
	return path
}

// isAbsWindows reports whether path, canonical or native, is absolute on
// Windows: a drive followed by a separator, a bare drive (the canonical drive
// root), or a UNC path. A path starting with one separator is relative to the
// current drive, so it is not absolute.
func isAbsWindows(path string) bool {
	switch {
	case len(path) >= 2 && isDriveLetter(path[0]) && path[1] == ':':
		return len(path) == 2 || isSeparator(path[2])
	case len(path) > 2:
		return isSeparator(path[0]) && isSeparator(path[1]) && !isSeparator(path[2])
	}
	return false
}

// systemBaseWindows returns the base of the system scope on Windows: root in
// image mode, else the SystemDrive value in canonical form, else C:.
func systemBaseWindows(root, systemDrive string) string {
	switch {
	case root != "":
		return root
	case systemDrive == "":
		return "C:"
	}
	return fromWindows(systemDrive)
}

// nativeWindows returns the native form of a path an error holds, which may
// be canonical, native or extended.
func nativeWindows(path string) string {
	return toWindows(fromWindows(path))
}

// isDriveLetter reports whether character is an ASCII letter.
func isDriveLetter(character byte) bool {
	return ('A' <= character && character <= 'Z') || ('a' <= character && character <= 'z')
}

// isSeparator reports whether character is a Windows path separator.
func isSeparator(character byte) bool {
	return character == '/' || character == '\\'
}

// rewrappedError stands in for a wrapper error of a type nativeErrors cannot
// rebuild: its text is the wrapper's with the inner error's text replaced by
// the rewritten one, and it unwraps to the rewritten chain.
type rewrappedError struct {
	text  string
	inner error
}

// Error returns the wrapper's text with the rewritten inner text.
func (e *rewrappedError) Error() string { return e.text }

// Unwrap returns the rewritten inner error.
func (e *rewrappedError) Unwrap() error { return e.inner }

// nativeErrors returns whether err's errors.Unwrap chain holds an
// *fs.PathError or *os.LinkError, and a copy of the chain with convert applied
// to every such path and link name. The input chain is never modified, since
// errors may be shared; a chain without those types comes back as it was.
func nativeErrors(err error, convert func(string) string) (bool, error) {
	switch typed := err.(type) {
	case *fs.PathError:
		_, inner := nativeErrors(typed.Err, convert)
		return true, &fs.PathError{Op: typed.Op, Path: convert(typed.Path), Err: inner}
	case *os.LinkError:
		_, inner := nativeErrors(typed.Err, convert)
		return true, &os.LinkError{Op: typed.Op, Old: convert(typed.Old), New: convert(typed.New),
			Err: inner}
	}
	inner := errors.Unwrap(err)
	if inner == nil {
		return false, err
	}
	changed, rewritten := nativeErrors(inner, convert)
	if !changed {
		return false, err
	}
	text := strings.Replace(err.Error(), inner.Error(), rewritten.Error(), 1)
	return true, &rewrappedError{text: text, inner: rewritten}
}
