package parser

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The three functions below are the only way a parser puts a secret into
// a row. Each returns the text as written, in every run, and records what
// in it is secret in the run's recorder, read from the artifact's home
// (input.Home.Secrets), as a secret of the parse running. Every row a
// parser returns is therefore the same with and without --no-secrets;
// the output replaces the recorded secrets where it is written
// (internal/secrets). A parser never tests the setting, never writes the
// placeholder and never hides a value by a wording of its own.

// SecretText returns a secret value read from the evidence (a token, an
// API key, a password, a cookie) as written, and records it. An empty
// value is no secret.
func SecretText(artifact *input.Artifact, value string) string {
	recorderOf(artifact).Value(value)
	return value
}

// URLText returns a URL, a host string or text holding URLs as written,
// and records the credentials of each URL that has a scheme: its user
// information (whole when it holds a colon, the password also on its own;
// alone only from secrets.EverywhereLength bytes), and the value of a
// query or fragment parameter named as a credential (token, key, sig,
// X-Amz-Signature and the others of the table in internal/secrets).
// Credentials written without a scheme are not identified. A URL written
// with JSON's escaped slashes (https:\/\/) is read too.
func URLText(artifact *input.Artifact, text string) string {
	recorderOf(artifact).URLs(text)
	return text
}

// ReaderErrorText returns the message of a JSON, TOML or XML reader's error
// for a file that holds secrets in full, and records the reader's detail,
// which can quote the file, so the output keeps where the reader stopped
// ("invalid JSON at line 3, column 9") and replaces the rest. A TOML
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
