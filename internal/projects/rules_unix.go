//go:build unix

package projects

import "github.com/seanthegeek/doubleagent/internal/fsmeta"

// platformRules returns the POSIX rules on a live run, where a drive value
// names no place: only a value starting with / is a path. Under an image
// root a drive value (a drive path, a drive URI, or a drive JSON key or
// string) is a path too, spelled as the disk holds it by a listing of each
// directory, behind check and through opener, since a disk that keeps case
// holds one spelling of a Windows name; every other value keeps the POSIX
// rule, so a Linux or macOS image reads as it does live.
func platformRules(root string, check RootExit, opener *fsmeta.Opener) pathRules {
	if root == "" {
		return pathRules{}
	}
	return pathRules{driveLetters: true, decodeEscapes: true, posixOthers: true, driveJSON: true,
		spell: newDiskSpeller(check, opener).spell}
}
