package tomldoc

// ErrorKind says why Parse refused a document.
type ErrorKind uint8

// The reasons Parse refuses a document.
const (
	// InvalidSyntax is a document that breaks the TOML rules.
	InvalidSyntax ErrorKind = iota
	// TooLarge is a document over maxDocumentBytes, refused unread.
	TooLarge
	// TooDeep is a document whose paths nest deeper than maxDepth, refused
	// before the reader sees it.
	TooDeep
)

// Error is the error Parse returns; errors.As finds it through any
// wrapping. Line and Column count from 1 and are 0 when not known: a
// TooLarge refusal has neither, a TooDeep refusal a line only, and an
// InvalidSyntax error both when the reader gave them. The message of a
// limit refusal (TooLarge, TooDeep) quotes nothing from the document, only
// numbers, so it is safe to show where the document may hold secrets; an
// InvalidSyntax message may quote the document.
type Error struct {
	Kind         ErrorKind
	Line, Column int
	text         string
	cause        error
}

// Error returns the message.
func (e *Error) Error() string { return e.text }

// Unwrap returns the reader's own error, or nil.
func (e *Error) Unwrap() error { return e.cause }

// IsLimit reports whether the document was refused for a limit of this
// package (TooLarge or TooDeep) rather than for breaking the TOML rules.
func (e *Error) IsLimit() bool { return e.Kind == TooLarge || e.Kind == TooDeep }
