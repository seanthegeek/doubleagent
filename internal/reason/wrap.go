package reason

// Wrap returns an error whose text is text and that wraps err, so
// errors.Is and errors.As still find err. It is for a message that words
// err its own way (Describe, a parser's problem row), where fmt.Errorf with
// %w would put err's own text in the message.
func Wrap(text string, err error) error {
	return &wrapped{text: text, err: err}
}

// wrapped is the error Wrap returns.
type wrapped struct {
	text string
	err  error
}

// Error returns the text given to Wrap.
func (w *wrapped) Error() string {
	return w.text
}

// Unwrap returns the error given to Wrap.
func (w *wrapped) Unwrap() error {
	return w.err
}
