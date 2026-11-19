package parser

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The two functions below are the only way a parser puts a secret into a
// row: a value secret because of its field (a token, key, password or
// cookie) and a reader's error that can quote a file holding secrets. Each
// returns the text as written in every run and records the secret in the
// run's recorder (input.Home.Secrets) as one of the parse running; a URL's
// credentials need no call, the output finds them in every string. Rows are
// the same with and without --no-secrets, and the output replaces secrets
// where it is written (internal/secrets). A parser never tests the setting,
// never writes the placeholder and never hides a value by its own wording.

// SecretText returns a secret value read from the evidence (a token, an
// API key, a password, a cookie) as written, and records it: one of
// secrets.EverywhereLength bytes or more is replaced in every string of the
// output, a shorter one only in the text of this parse's rows. An empty
// value is no secret.
func SecretText(artifact *input.Artifact, value string) string {
	recordForms(recorderOf(artifact), value)
	return value
}

// SecretJSONText returns a secret value read from a JSON document as row
// text (Text: a string as it is, an array or object as compact JSON, a
// headers object for one) and records it whole and, inside an array or
// object, each string in it as SecretText does and as the compact JSON
// writes it when that differs, so each is replaced wherever it stands.
func SecretJSONText(artifact *input.Artifact, value jsonvalue.Value) string {
	text := Text(value)
	recorder := recorderOf(artifact)
	recordForms(recorder, text)
	if !value.IsString() {
		recordStrings(recorder, value)
	}
	return text
}

// recordStrings records each string inside value, an array or object: as
// it is and as compact JSON writes it inside quotes.
func recordStrings(recorder *secrets.Recorder, value jsonvalue.Value) {
	for _, item := range value.Items() {
		recordStrings(recorder, item)
	}
	for _, member := range value.Members() {
		recordStrings(recorder, member.Value)
	}
	if value.IsString() {
		recordForms(recorder, value.Text())
		quoted := value.JSON()
		recordForms(recorder, quoted[1:len(quoted)-1])
	}
}

// recordForms records a secret value in each form a row can hold it: as it
// is, without the white space around it, and, for an HTTP authorization
// value of the Bearer (RFC 6750) or Basic (RFC 7617) scheme, its
// credentials after the scheme, which are the secret a log or another
// header shows alone.
func recordForms(recorder *secrets.Recorder, value string) {
	recorder.Value(value)
	trimmed := strings.TrimSpace(value)
	recorder.Value(trimmed)
	if scheme, credentials, found := strings.Cut(trimmed, " "); found &&
		(strings.EqualFold(scheme, "Bearer") || strings.EqualFold(scheme, "Basic")) {
		recorder.Value(strings.TrimSpace(credentials))
	}
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
// to replace in it.
func Within(artifact *input.Artifact, produce func(emit Emit) error, emit Emit) (*secrets.Found, error) {
	recorder := recorderOf(artifact)
	found := recorder.Begin()
	defer recorder.End()
	err := produce(func(row timeline.Row) {
		row.Secrets = found
		emit(row)
	})
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
