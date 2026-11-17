package manifest

// platformFields holds the two fields the PowerShell collector writes after
// mode: owner and attributes, each written even when empty.
type platformFields struct {
	Owner      string `json:"owner"`
	Attributes string `json:"attributes"`
}

// platformOf returns the platform fields of row.
func platformOf(row Row) platformFields {
	return platformFields{Owner: row.Owner, Attributes: row.Attributes}
}
