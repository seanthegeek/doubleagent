package projects

// platformRules returns ConvertTo-LocalPath's rules: drive paths and file://
// URIs are paths, and a POSIX path names a place only inside an image.
func platformRules() pathRules {
	return pathRules{driveLetters: true, decodeEscapes: true, dropPOSIXLive: true}
}
