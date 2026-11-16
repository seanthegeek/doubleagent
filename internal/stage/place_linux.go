package stage

import "os"

// reportedPath returns "": Linux gives no exact path for an open file, so a
// staged member's spelling comes from a walk of the tree (Stager.member).
func reportedPath(*os.File) string {
	return ""
}
