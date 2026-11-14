//go:build unix

package manifest

// appendPlatformFields adds nothing: Unix rows have no owner or attributes
// fields.
func appendPlatformFields(out []byte, _ Row) []byte { return out }
