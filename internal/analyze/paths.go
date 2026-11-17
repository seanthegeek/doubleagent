package analyze

import "strings"

// posixPathText returns path as Python's str(PurePosixPath(path)) prints
// it: repeated slashes and "." components removed, ".." kept, no trailing
// slash, exactly two leading slashes kept, "." for an empty path.
func posixPathText(path string) string {
	root := ""
	switch {
	case strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///"):
		root = "//"
	case strings.HasPrefix(path, "/"):
		root = "/"
	}
	parts := pathParts(path[len(root):], "/")
	if root == "" && len(parts) == 0 {
		return "."
	}
	return root + strings.Join(parts, "/")
}

// windowsPathText returns path as Python's str(PureWindowsPath(path))
// prints it: slashes turned into backslashes, the drive (C: or a UNC
// \\server\share) and root kept, repeated separators and "." components
// removed, ".." kept, "." for an empty path.
func windowsPathText(path string) string {
	path = strings.ReplaceAll(path, "/", `\`)
	drive := ""
	switch {
	case strings.HasPrefix(path, `\\`):
		server, rest, _ := strings.Cut(path[2:], `\`)
		share, _, _ := strings.Cut(rest, `\`)
		drive = `\\` + server + `\` + share
		if share == "" {
			drive = `\\` + server + `\`
		}
		drive = path[:min(len(drive), len(path))]
	case len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0]):
		drive = path[:2]
	}
	rest, root := path[len(drive):], ""
	if strings.HasPrefix(rest, `\`) || strings.HasPrefix(drive, `\\`) {
		root = `\`
	}
	parts := pathParts(rest, `\`)
	if drive == "" && root == "" && len(parts) == 0 {
		return "."
	}
	return drive + root + strings.Join(parts, `\`)
}

// pathParts returns the components of text split at separator, without
// empty and "." ones.
func pathParts(text, separator string) []string {
	var parts []string
	for _, part := range strings.Split(text, separator) {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

// isASCIILetter reports whether character is an ASCII letter.
func isASCIILetter(character byte) bool {
	return 'a' <= character|0x20 && character|0x20 <= 'z'
}
