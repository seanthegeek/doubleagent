//go:build unix

package fsmeta

// Owners names the owners of files. On Unix the manifest has no owner
// field, so it holds nothing.
type Owners struct{}

// NewOwners returns an Owners for one run.
func NewOwners() *Owners {
	return &Owners{}
}

// Name returns "": Unix rows record the owner as uid and gid instead.
func (o *Owners) Name(string) string {
	return ""
}
