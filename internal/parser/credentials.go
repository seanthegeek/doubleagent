package parser

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// The three functions below are the only way a parser puts a secret into
// a row. Each returns the text the row shows under the run's setting, which
// it reads from the artifact's home (input.Home.Secrets): by default the
// evidence as written, and with --no-secrets the placeholder in place of
// the secret (internal/secrets says which parts are secrets). A parser
// never tests the setting or writes the placeholder itself. An artifact
// without a home, as a test may build, shows secrets.

// SecretText returns a secret value read from the evidence (a token, an
// API key, a password, a cookie) as a row shows it: as written, or the
// placeholder when the run withholds secrets. An empty value stays empty.
func SecretText(artifact *input.Artifact, value string) string {
	return settingOf(artifact).Value(value)
}

// URLText returns a URL, a host string or text holding URLs (a command
// line, a JSON document) as a row shows it: as written, or with the
// credentials of each URL replaced by the placeholder when the run
// withholds secrets: the user information before the host, user:password@
// written without a scheme, and the value of a query or fragment parameter
// named as a credential (token, key, api_key, secret, password and the
// others secrets.Setting.URLs lists). The rest of each URL is kept.
func URLText(artifact *input.Artifact, text string) string {
	return settingOf(artifact).URLs(text)
}

// ReaderErrorText returns the message of a JSON, TOML or XML reader's error
// for a file that holds secrets, whose detail can quote the file, as a row
// shows it: in full, or where the reader stopped ("invalid JSON at line 3,
// column 9") followed by the placeholder when the run withholds secrets. A
// TOML document refused for a size or depth limit quotes nothing and is
// given in full either way.
func ReaderErrorText(artifact *input.Artifact, err error) string {
	return settingOf(artifact).ReaderError(err)
}

// settingOf returns the secrets setting of the artifact's home, the zero
// setting (show) for an artifact without one.
func settingOf(artifact *input.Artifact) secrets.Setting {
	if artifact == nil || artifact.Home == nil {
		return secrets.Setting{}
	}
	return artifact.Home.Secrets
}
