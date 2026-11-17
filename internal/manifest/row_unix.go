//go:build unix

package manifest

// platformFields holds the fields only some platforms write: none on Unix,
// where rows have no owner or attributes.
type platformFields struct{}

// platformOf returns the platform fields of row.
func platformOf(Row) platformFields { return platformFields{} }
