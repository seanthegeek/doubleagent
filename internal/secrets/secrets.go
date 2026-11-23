package secrets

// DefaultPlaceholder is the text a withheld secret is replaced by unless
// the analyst names another.
const DefaultPlaceholder = "REDACTED"

// EverywhereLength is the length in bytes from which a secret value or a
// URL credential is replaced in every string of the output, not only in the
// text of the rows of the parse that identified it or where the URL holds
// it. A shorter secret (a one-letter password, a three-digit code) would
// replace the same letters wherever they stand in unrelated text; eight
// bytes is the shortest of the generated tokens and keys the agents store.
const EverywhereLength = 8

// Setting is how one analysis run shows secrets. Its zero value shows
// them; with Withhold set, each identified secret is replaced by
// Placeholder, which may be empty.
type Setting struct {
	Withhold    bool
	Placeholder string
}
