package parser

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/secrets/urlcredentials"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The functions below are the only way a parser puts a secret into a row:
// a value secret because of its field (a token, key, password or cookie),
// as text or as JSON, and a reader's error that can quote a file holding
// secrets. Each
// returns the text as written in every run and records the secret in the
// run's recorder (input.Home.Secrets) as one of the parse running; a URL's
// credentials need no call, the output finds them in every string. Rows are
// the same with and without --no-secrets, and the output replaces secrets
// where it is written (internal/secrets). A parser never tests the setting,
// never writes the placeholder and never hides a value by its own wording.

// SecretText returns a secret value read from the evidence (a token, an
// API key, a password, a cookie) as written, and records it in each form a
// row can hold it (recordForms). The secret itself, the value without the
// white space around it or, after a Bearer or Basic scheme, the credentials
// that follow, decides where every form is replaced: in every string of the
// output when it is secrets.EverywhereLength bytes or more, else only in
// the text of this parse's rows. A value that is empty once the white space
// around it is removed is no secret.
func SecretText(artifact *input.Artifact, value string) string {
	recordForms(recorderOf(artifact), value)
	return value
}

// SecretJSONText returns a secret value read from a JSON document as row
// text (Text: a string as it is, an array or object as compact JSON, a
// headers object for one) and records it whole as SecretText does. Inside
// an array or object, a string under a member that names a credential
// (credentialName) is recorded as SecretText records a value, so it is
// replaced where another string quotes it alone. Any other string, such as
// the value of a Content-Type or X-Version header, is no secret on its own
// and is replaced only as part of the whole text, so a short one never
// changes the rest of the row. Numbers and booleans are not recorded.
func SecretJSONText(artifact *input.Artifact, value jsonvalue.Value) string {
	text := Text(value)
	recorder := recorderOf(artifact)
	recordForms(recorder, text)
	if !value.IsString() {
		recordStrings(recorder, value, "")
	}
	return text
}

// recordStrings records each string inside value, an array or object,
// whose name names a credential: the name of the member that holds it, or
// of the nearest member around an array that does.
func recordStrings(recorder *secrets.Recorder, value jsonvalue.Value, name string) {
	for _, item := range value.Items() {
		recordStrings(recorder, item, name)
	}
	for _, member := range value.Members() {
		recordStrings(recorder, member.Value, member.Key)
	}
	if value.IsString() && credentialName(name) {
		recordForms(recorder, value.Text())
	}
}

// credentialName reports whether a JSON member's name, ASCII case
// ignored, names a credential: a parameter whose value is one
// (urlcredentials.IsCredentialName), or an HTTP header that carries one:
// Authorization and Proxy-Authorization (RFC 9110 sections 11.6.2 and
// 11.7.2), Cookie and Set-Cookie (RFC 6265 sections 5.4 and 4.1), and
// X-API-Key, the header the Anthropic API and Amazon API Gateway read an
// API key from.
func credentialName(name string) bool {
	for _, header := range []string{"authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key"} {
		if strings.EqualFold(name, header) {
			return true
		}
	}
	return urlcredentials.IsCredentialName(name)
}

// recordForms records a secret value in each form a row can hold it: as
// written, without the white space around it, and, for a Bearer (RFC 6750)
// or Basic (RFC 7617) authorization value, its credentials after the
// scheme; and each of those in the spellings of jsonSpellings, at most
// seven texts each, 21 in all. The last unescaped form is the secret
// itself, and its length alone decides the reach of every form, so a
// longer spelling reaches no further than the secret.
func recordForms(recorder *secrets.Recorder, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	forms := []string{value, trimmed}
	if scheme, credentials, found := strings.Cut(trimmed, " "); found &&
		(strings.EqualFold(scheme, "Bearer") || strings.EqualFold(scheme, "Basic")) {
		forms = append(forms, strings.TrimSpace(credentials))
	}
	record := recorder.Value
	if len(forms[len(forms)-1]) < secrets.EverywhereLength {
		record = recorder.RowValue
	}
	for _, form := range forms {
		record(form)
		for _, spelling := range jsonSpellings(form) {
			if spelling != form {
				record(spelling)
			}
		}
	}
}

// htmlEscapedLower and htmlEscapedUpper write the characters &, < and > as
// Go's encoding/json does by default, with the hex digits in lower case,
// and in upper case.
var (
	htmlEscapedLower = strings.NewReplacer("&", `\u0026`, "<", `\u003c`, ">", `\u003e`)
	htmlEscapedUpper = strings.NewReplacer("&", `\u0026`, "<", `\u003C`, ">", `\u003E`)
)

// jsonSpellings returns the ways a JSON writer can write text inside the
// quotes of a string, each once, in no set order: as compact JSON writes it
// (backslash, quote and control characters escaped, U+2028 and U+2029 as
// \u2028 and \u2029, nothing else); and, since another writer escapes a
// slash as \/ (PHP's json_encode) and the characters &, < and > as \u0026,
// \u003c and \u003e (Go's encoding/json by default, in either hex case), that
// text with each slash escaped, with each of those three characters escaped
// in either hex case, and with both. At most six texts.
func jsonSpellings(text string) []string {
	quoted := jsonvalue.NewString(text).JSON()
	plain := quoted[1 : len(quoted)-1]
	slashed := strings.ReplaceAll(plain, "/", `\/`)
	spellings := []string{plain, slashed}
	for _, base := range []string{plain, slashed} {
		spellings = append(spellings, htmlEscapedLower.Replace(base), htmlEscapedUpper.Replace(base))
	}
	slices.Sort(spellings)
	return slices.Compact(spellings)
}

// ReaderErrorText returns the message of a JSON, TOML or XML reader's error
// for a file that holds secrets in full, and records the reader's detail,
// which can quote the file, so the output keeps where the reader stopped
// ("invalid JSON at line 3, column 9") and replaces the rest in the text of
// this parse's rows and in its problem lines, nowhere else. A TOML
// document refused for a size or depth limit quotes nothing and records
// nothing.
func ReaderErrorText(artifact *input.Artifact, err error) string {
	recorderOf(artifact).ReaderError(err)
	return err.Error()
}

// Collect runs one parse of artifact by each as a run does (Within).
func Collect(each Parser, artifact *input.Artifact, options Options, emit Emit) (*secrets.Found, error) {
	return Within(artifact, func(inner Emit) error { return each.Parse(artifact, options, inner) }, emit)
}

// Within runs produce as one parse of artifact: what the kit's functions
// identify while it runs is recorded as the parse's own, which it returns
// (nil in a run that shows secrets) with produce's error, and each row
// produce emits carries that record (timeline.Row.Secrets) for the output
// to replace in it. Every field of a row but its text that is longer than
// CarriedLimit is cut to it, and the parse ends with one problem row per
// field cut (CarriedProblem), so no parser can leave one long.
func Within(artifact *input.Artifact, produce func(emit Emit) error, emit Emit) (*secrets.Found, error) {
	recorder := recorderOf(artifact)
	found := recorder.Begin()
	defer recorder.End()
	fields := carriedFields{recorder: recorder}
	err := produce(func(row timeline.Row) {
		row.Secrets = found
		fields.cut(&row)
		emit(row)
	})
	for _, row := range fields.problemRows() {
		row.Secrets = found
		emit(row)
	}
	return found, err
}

// recorderOf returns the recorder of the artifact's home, nil (record
// nothing) for an artifact without one.
func recorderOf(artifact *input.Artifact) *secrets.Recorder {
	if artifact == nil || artifact.Home == nil {
		return nil
	}
	return artifact.Home.Secrets
}
