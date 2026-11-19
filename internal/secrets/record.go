package secrets

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/secrets/urlcredentials"
	"github.com/seanthegeek/doubleagent/internal/textset"
)

// Recorder is one run's record of the secrets identified in it, and the
// functions that replace them in the output (Text, RowText, ProblemText).
// A nil Recorder is a run that shows secrets: it records nothing and each
// function returns the text as written.
type Recorder struct {
	setting Setting
	// current is the record of the parse running, nil between parses;
	// outer holds the records of the parses it started inside, innermost
	// last; everywhere holds every secret replaced in every string.
	current, everywhere *Found
	outer               []*Found
	work                textset.Counter
	// joined is the matcher replace last built for the records a row
	// lists, kept for the next row that lists the same ones.
	joined joinedMatcher
}

// NewRecorder returns the recorder of a run with setting, nil when the
// setting shows secrets.
func NewRecorder(setting Setting) *Recorder {
	if !setting.Withhold {
		return nil
	}
	return &Recorder{setting: setting, everywhere: &Found{}}
}

// Setting returns the run's setting, the zero setting for a nil Recorder.
func (r *Recorder) Setting() Setting {
	if r == nil {
		return Setting{}
	}
	return r.setting
}

// Begin starts one parse: what is identified until End is the parse's, the
// Found it returns, which the rows of the parse carry to the output. A
// parse begun inside another is the inner one until it ends. It is nil for
// a nil Recorder.
func (r *Recorder) Begin() *Found {
	if r == nil {
		return nil
	}
	r.outer = append(r.outer, r.current)
	r.current = &Found{}
	return r.current
}

// End ends the parse Begin last started; the parse it began inside, if
// any, is running again.
func (r *Recorder) End() {
	if r == nil || len(r.outer) == 0 {
		return
	}
	r.current = r.outer[len(r.outer)-1]
	r.outer = r.outer[:len(r.outer)-1]
}

// CountWork makes the recorder count in work the bytes it reads finding
// URLs, building matchers and searching texts, for a test that holds the
// cost of writing an output to a bound by a count.
func (r *Recorder) CountWork(work textset.Counter) {
	if r != nil {
		r.work = work
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

// EverywhereTexts returns the texts the run replaces in every string, as
// its placeholder matches them (with an empty one, a URL's user
// information with its @ too), sorted, for a test that checks the output
// against them; nil for a nil Recorder.
func (r *Recorder) EverywhereTexts() []string {
	if r == nil {
		return nil
	}
	var texts []string
	for key, each := range r.everywhere.forms {
		if r.setting.Placeholder == "" || each.plain() {
			texts = append(texts, key.text)
		}
	}
	slices.Sort(texts)
	return slices.Compact(texts)
}

// Value records a secret value read from the evidence (a token, key,
// password or cookie). One of EverywhereLength bytes or more is replaced in
// every string of the output; a shorter one only in the text of the rows
// of the parse running (RowText), so a one-letter password never touches a
// timestamp, a host or a session id. Outside a parse a shorter one is
// replaced nowhere. A value that is empty once the white space around it
// is removed is no secret.
func (r *Recorder) Value(value string) {
	if r == nil || strings.TrimSpace(value) == "" {
		return
	}
	secret := form{secret: value}
	if len(value) >= EverywhereLength {
		r.everywhere.add(secret, everyString)
		r.current.add(secret, listedOnly)
		return
	}
	r.current.add(secret, rowText)
}

// RowValue records a value replaced only in the text of the rows of the
// parse running, whatever its own length: a form of a secret shorter than
// EverywhereLength (the secret with white space around it, with its Bearer
// or Basic scheme, or JSON-escaped, as parser.SecretText records it), which
// reaches no further than the secret itself. A value that is empty once the
// white space around it is removed is no secret.
func (r *Recorder) RowValue(value string) {
	if r == nil || strings.TrimSpace(value) == "" {
		return
	}
	r.current.add(form{secret: value}, rowText)
}

// Collect reads one string the output will write for the credentials of
// the URLs in it (identifyURLs says which) and records each of
// EverywhereLength bytes or more as replaced everywhere. A command collects
// every string of its output before it writes any, so such a credential is
// replaced in every string, not only where the URL holds it; Text replaces
// a shorter one where the URL holds it only. A slash in a credential is
// written \/ where JSON escapes it, so both forms are recorded, and the
// length of the credential with its slashes unescaped decides for both
// (URLCredentialEverywhere).
func (r *Recorder) Collect(text string) {
	if r == nil {
		return
	}
	identifyURLs(text, func(secret urlcredentials.Credential) {
		value := text[secret.Start:secret.End]
		if URLCredentialEverywhere(value) {
			decoded := strings.ReplaceAll(value, `\/`, "/")
			for _, each := range []string{value, decoded, strings.ReplaceAll(decoded, "/", `\/`)} {
				r.everywhere.add(form{secret: each}, everyString)
			}
			if secret.After > 0 {
				r.everywhere.add(form{secret: value, after: text[secret.End : secret.End+secret.After]},
					everyString)
			}
		}
	}, r.work)
}

// URLCredentialEverywhere reports whether Collect records a URL credential
// as written in a text to be replaced in every string: when it is
// EverywhereLength bytes or more with each \/ read as /.
func URLCredentialEverywhere(credential string) bool {
	return len(strings.ReplaceAll(credential, `\/`, "/")) >= EverywhereLength
}

// ReaderError records the detail of err, the error a JSON, TOML or XML
// reader gave for a file that may hold secrets, which can quote the file:
// the text after the position the reader names ("invalid JSON at line 3,
// column 9: "), or the whole message of an error of another kind. The
// detail is often the reader's own wording ("unexpected end of JSON
// input"), so whatever its length it is replaced only in the rows and the
// problem lines of the parse running, never elsewhere. A TOML document
// refused for a size or depth limit (tomldoc.Error.IsLimit) quotes nothing
// from the file and records nothing.
func (r *Recorder) ReaderError(err error) {
	if r == nil || err == nil || isLimit(err) {
		return
	}
	message := err.Error()
	if !isReaderError(err) {
		r.current.add(form{secret: message}, parseProblem)
		return
	}
	if _, detail, found := strings.Cut(message, ": "); found {
		r.current.add(form{secret: detail}, parseProblem)
		// With an empty placeholder the ": " before the detail goes too.
		r.current.add(form{before: ": ", secret: detail}, parseProblem)
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

// reach is where the output replaces a recorded text.
type reach uint8

const (
	// everyString is every string of the output: a secret of
	// EverywhereLength bytes or more, recorded in the run's everywhere.
	everyString reach = iota
	// rowText is the text of the rows of the parse that recorded it: a
	// secret value shorter than EverywhereLength (Value), or any form of
	// one (RowValue).
	rowText
	// parseProblem is the text of the rows and the problem lines of the
	// parse that recorded it: a reader's detail, whatever its length.
	parseProblem
	// listedOnly is a secret the parse recorded that everywhere replaces;
	// the parse lists it (Secrets) and matches nothing for it.
	listedOnly
	reaches
)

// placed is a recorded text and where it is replaced.
type placed struct {
	text  string
	where reach
}

// Found is what one parse identified, or, for the run's everywhere, the
// secrets replaced in every string; or, made by WithDuplicates, the list of
// the parses' records a row kept for its dropped duplicates stands for,
// which records nothing of its own.
type Found struct {
	// forms holds each form by the text it matches and where; texts lists
	// the texts of each reach in the order recorded, and size their bytes.
	forms map[placed]form
	texts [reaches][]string
	size  [reaches]int
	// matchers holds the matcher of each reach's texts, built when first
	// needed and again once more are recorded (matched counts them).
	matchers [reaches]*textset.Set
	matched  [reaches]int
	// listed holds the parses' records a list WithDuplicates made stands
	// for, nil for a parse's own record.
	listed []*Found
}

// add records one form replaced in where; of a text recorded both plain
// and with a part, the plain form is kept, since it withholds more. A nil
// Found (no parse running) records nothing.
func (f *Found) add(secret form, where reach) {
	if f == nil {
		return
	}
	key := placed{secret.text(), where}
	if f.forms == nil {
		f.forms = map[placed]form{}
	}
	if earlier, ok := f.forms[key]; ok {
		if !earlier.plain() {
			f.forms[key] = secret
		}
		return
	}
	f.forms[key] = secret
	f.texts[where] = append(f.texts[where], key.text)
	f.size[where] += len(key.text)
}

// Secrets returns the distinct secrets recorded, sorted; nil for a nil
// Found.
func (f *Found) Secrets() []string {
	return f.secretsIn(func(reach) bool { return true })
}

// Values returns the distinct secret values recorded (Value and RowValue),
// whether they are replaced in every string or in the parse's rows alone,
// sorted; nil for a nil Found.
func (f *Found) Values() []string {
	return f.secretsIn(func(where reach) bool { return where == rowText || where == listedOnly })
}

// EverywhereValues returns the distinct secret values recorded (Value) that
// the run replaces in every string, sorted; nil for a nil Found.
func (f *Found) EverywhereValues() []string {
	return f.secretsIn(func(where reach) bool { return where == listedOnly })
}

// ReaderDetails returns the distinct readers' details recorded, sorted,
// which the output replaces in the parse's rows and problem lines whatever
// their length; nil for a nil Found.
func (f *Found) ReaderDetails() []string {
	return f.secretsIn(func(where reach) bool { return where == parseProblem })
}

// secretsIn returns the distinct secrets of the forms recorded in a reach
// that holds, sorted; nil for a nil Found.
func (f *Found) secretsIn(holds func(reach) bool) []string {
	if f == nil {
		return nil
	}
	unique := map[string]bool{}
	for _, parse := range f.Records() {
		for key, each := range parse.forms {
			if holds(key.where) {
				unique[each.secret] = true
			}
		}
	}
	secrets := make([]string, 0, len(unique))
	for secret := range unique {
		secrets = append(secrets, secret)
	}
	slices.Sort(secrets)
	return secrets
}

// WithDuplicates returns the record of a row kept for rows deduplication
// dropped as its duplicates: a short list of own and each of dropped (a
// parse's record, or one WithDuplicates made), each parse's record once,
// so the row's text is replaced for each parse as that parse's rows are.
// The records are listed, not copied, so the list costs its length however
// much each record holds. It is own when nothing else is listed, and nil
// when nothing is.
func WithDuplicates(own *Found, dropped []*Found) *Found {
	var listed []*Found
	seen := map[*Found]bool{}
	for _, record := range append([]*Found{own}, dropped...) {
		for _, parse := range record.Records() {
			if !seen[parse] {
				seen[parse] = true
				listed = append(listed, parse)
			}
		}
	}
	switch {
	case len(listed) == 0:
		return nil
	case len(listed) == 1:
		return listed[0]
	}
	return &Found{listed: listed}
}

// Records returns the records of the parses f stands for: f itself, or
// the list WithDuplicates made; nil for a nil Found.
func (f *Found) Records() []*Found {
	switch {
	case f == nil:
		return nil
	case f.listed != nil:
		return f.listed
	}
	return []*Found{f}
}
