package tomldoc

// Error is the error Parse returns; errors.As finds it through any
// wrapping. The message of a limit refusal (a document too large or nested
// too deep) quotes nothing from the document, only numbers, so it is safe
// to show where the document may hold secrets; the message of a document
// that breaks the TOML rules may quote the document.
type Error struct {
	limit bool
	text  string
	cause error
}

// Error returns the message.
func (e *Error) Error() string { return e.text }

// Unwrap returns the reader's own error, or nil.
func (e *Error) Unwrap() error { return e.cause }

// IsLimit reports whether the document was refused for a limit of this
// package rather than for breaking the TOML rules.
func (e *Error) IsLimit() bool { return e.limit }
