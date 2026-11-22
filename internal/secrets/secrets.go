// Package secrets identifies the secrets in the evidence and replaces them
// where the analysis output is written: a token, key, password or cookie
// value, the credentials a URL carries, and the detail of a reader's error
// for a file that holds secrets, which can quote the file. By default the
// output shows each as the evidence holds it, since it helps establish what
// an agent reached. When the analyst asks to withhold secrets (timeline
// --no-secrets, detect --no-secrets), each is replaced by one placeholder,
// REDACTED unless the analyst names another.
//
// Identifying and replacing are apart. A value secret by its field, or a
// reader's detail, is identified by the parser through the parser kit
// (parser.SecretText, parser.ReaderErrorText), which records it in the
// run's Recorder and returns the text as written; a URL's credentials are
// found by the output stage in every string it writes (package
// urlcredentials holds the grammar it reads URLs by). Every
// row is the same in both modes. Replacement happens once, where a command
// writes its output, each kind reaching its own strings: a value of
// EverywhereLength bytes or more every string, a shorter one only the text
// of the rows of its parse (RowText); a reader's detail, whatever its
// length, only the text of the rows and the problem lines of its parse
// (ProblemText); a URL's credentials the bytes where the URL holds them,
// and every string too when EverywhereLength bytes or more.
//
// Withholding covers only what is identified. A credential written in free
// text that nothing identifies (a prompt, a command line, a tool's output, a
// pasted file) is shown either way, so a person reviews a timeline before
// it is shared, whichever mode wrote it.
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
