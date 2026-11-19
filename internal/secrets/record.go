package secrets

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textset"
)

// Recorder is one run's record of the secrets identified in it, and the
// one function that replaces them in the output (Text). A nil Recorder is
// a run that shows secrets: it records nothing and Text returns the text
// as written.
type Recorder struct {
	setting Setting
	// current is the parse running, command what is identified outside
	// any parse (a command's own reading, such as a repository's remotes),
	// and everywhere every secret of EverywhereLength bytes or more.
	current, command, everywhere *Found
	work                         textset.Counter
}

// NewRecorder returns the recorder of a run with setting, nil when the
// setting shows secrets.
func NewRecorder(setting Setting) *Recorder {
	if !setting.Withhold {
		return nil
	}
	return &Recorder{setting: setting, command: &Found{}, everywhere: &Found{}}
}

// Setting returns the run's setting, the zero setting for a nil Recorder.
func (r *Recorder) Setting() Setting {
	if r == nil {
		return Setting{}
	}
	return r.setting
}

// Begin starts one parse: what is identified until End is the parse's, the
// Found it returns, which the rows of the parse carry to the output. It is
// nil for a nil Recorder.
func (r *Recorder) Begin() *Found {
	if r == nil {
		return nil
	}
	r.current = &Found{}
	return r.current
}

// End ends the parse Begin started; what is identified next is the
// command's own.
func (r *Recorder) End() {
	if r != nil {
		r.current = nil
	}
}

// Current returns the record of the parse running, nil when none is, for a
// test that looks at a parse's rows as they are emitted.
func (r *Recorder) Current() *Found {
	if r == nil {
		return nil
	}
	return r.current
}

// Command returns the secrets identified outside any parse, which the
// command's own output (its summary, inventory.json, detect's report)
// replaces; nil for a nil Recorder.
func (r *Recorder) Command() *Found {
	if r == nil {
		return nil
	}
	return r.command
}

// Value records a secret value read from the evidence (a token, key,
// password or cookie). An empty value is no secret.
func (r *Recorder) Value(value string) {
	r.add(form{secret: value})
}

// URLs records the credentials of each URL in text (identifyURLs says
// which).
func (r *Recorder) URLs(text string) {
	if r != nil {
		identifyURLs(text, r.add, r.work)
	}
}

// ReaderError records the detail of err, the error a JSON, TOML or XML
// reader gave for a file that may hold secrets, which can quote the file:
// the text after the position the reader names ("invalid JSON at line 3,
// column 9: "), or the whole message of an error of another kind. A TOML
// document refused for a size or depth limit (tomldoc.Error.IsLimit)
// quotes nothing from the file and records nothing.
func (r *Recorder) ReaderError(err error) {
	if r == nil || err == nil {
		return
	}
	if isLimit(err) {
		return
	}
	message := err.Error()
	if !isReaderError(err) {
		r.add(form{secret: message})
		return
	}
	if _, detail, found := strings.Cut(message, ": "); found {
		r.add(form{secret: detail})
		// With an empty placeholder the ": " before the detail goes too.
		r.add(form{before: ": ", secret: detail})
	}
}

// add records one form of a secret in the parse running, or the command's
// record, and in everywhere when the secret is long enough. An empty
// secret is no secret.
func (r *Recorder) add(secret form) {
	if r == nil || secret.secret == "" {
		return
	}
	scope := r.current
	if scope == nil {
		scope = r.command
	}
	scope.add(secret)
	if len(secret.secret) >= EverywhereLength {
		r.everywhere.add(secret)
	}
}

// form is one way a secret stands in the text: the secret alone, or with
// the text just before or after it (": " before a reader's detail, "@"
// after a URL's user information) that an empty placeholder takes away with
// it. A placeholder that is not empty replaces only the secret, so it keeps
// those parts as written and only the plain forms are matched.
type form struct {
	before, secret, after string
}

// text returns the text the form matches.
func (f form) text() string {
	return f.before + f.secret + f.after
}

// plain reports whether the form is the secret alone.
func (f form) plain() bool {
	return f.before == "" && f.after == ""
}

// Found is what one parse, or a command outside any parse, identified.
type Found struct {
	// forms holds each form by the text it matches; all lists the texts in
	// the order recorded, and local those of the secrets shorter than
	// EverywhereLength.
	forms      map[string]form
	all, local []string
	// matcher is built over local (over every text for everywhere) when
	// first needed, and again once more are recorded.
	matcher      *textset.Set
	matcherTexts int
}

// add records one form; of a text recorded both plain and with a part, the
// plain form is kept, since it withholds more.
func (f *Found) add(secret form) {
	text := secret.text()
	if f.forms == nil {
		f.forms = map[string]form{}
	}
	if earlier, ok := f.forms[text]; ok {
		if !earlier.plain() {
			f.forms[text] = secret
		}
		return
	}
	f.forms[text] = secret
	f.all = append(f.all, text)
	if len(secret.secret) < EverywhereLength {
		f.local = append(f.local, text)
	}
}

// Secrets returns the distinct secrets recorded, sorted; nil for a nil
// Found.
func (f *Found) Secrets() []string {
	if f == nil {
		return nil
	}
	unique := map[string]bool{}
	for _, each := range f.forms {
		unique[each.secret] = true
	}
	secrets := make([]string, 0, len(unique))
	for secret := range unique {
		secrets = append(secrets, secret)
	}
	slices.Sort(secrets)
	return secrets
}
