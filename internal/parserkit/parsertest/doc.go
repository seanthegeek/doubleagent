// Package parsertest runs a parser in a test as an analysis run does under
// a secrets setting: the parse records what the kit identifies in the
// recorder of the artifact's home, and the test receives the rows as
// timeline.jsonl writes them (timeline.Row.Withheld), each shared text
// written out in the order the parse emitted its rows (timeline.Resolve),
// so a parser's test of a withheld secret checks the text the output
// holds.
//
// A run that withholds also proves, for every parse a test makes this way,
// the rule the design rests on. The parse is run a second time showing
// secrets, and the rows, and the sessions built from them, must be the same
// in both modes, field for field and in order. The rows written must be the
// rows with exactly these replaced, each where it reaches, as a separate
// plain search gives them, in every string and with any placeholder (the
// empty one included): the recorded secrets, the credentials of their URLs
// and the kept part of each secret a cut value ran through
// (secrets.Recorder.Cut). No secret the run replaces in every string may
// remain. A parse run by Parse or Run must identify a secret, one run by
// ParseWithoutSecrets none. Only tests import it.
package parsertest
