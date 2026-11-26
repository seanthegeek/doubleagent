package projects

import "github.com/seanthegeek/doubleagent/internal/fsmeta"

// platformRules returns the drive-letter rules: drive paths and file://
// URIs are paths, also as JSON keys and strings, and a POSIX path names a
// place only inside an image. The file system spells each path as it holds
// it (fsmeta.OnDiskPath), so root, check and opener are not needed.
func platformRules(string, RootExit, *fsmeta.Opener) pathRules {
	return pathRules{driveLetters: true, decodeEscapes: true, dropPOSIXLive: true, driveJSON: true}
}
