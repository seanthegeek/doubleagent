package projects

// platformRules returns ConvertTo-LocalPath's rules: drive paths and file://
// URIs are paths, also as JSON keys and strings, and a POSIX path names a
// place only inside an image.
func platformRules() pathRules {
	return pathRules{driveLetters: true, decodeEscapes: true, dropPOSIXLive: true, driveJSON: true}
}
