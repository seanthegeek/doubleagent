//go:build parity

package replay

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The recordings come from an analyzer that always withheld the secrets of
// the sites the binary shows by default, each site in its own form. So the
// replay runs every parser case and every detect and timeline command case
// with secrets withheld (withheldSecrets, withSecretsWithheld), and the
// binary's placeholder form is compared with the recording's form through
// difference 230.

// replayPlaceholder is the placeholder every replayed parse and command
// withholds secrets as. It is not the default one, and no recorded text
// holds it, so a parser that writes the default placeholder itself instead
// of identifying the secret gives a row no entry of differences 230 and
// 231 takes, and the case fails.
const replayPlaceholder = "WITHHELD_BY_REPLAY"

// withheldSecrets returns the setting every replayed parser case runs
// under: secrets withheld as replayPlaceholder, as a run with --no-secrets
// --redaction-placeholder gives.
func withheldSecrets() secrets.Setting {
	return secrets.Setting{Withhold: true, Placeholder: replayPlaceholder}
}

// parseWithheld runs one parse of artifact as a run with --no-secrets does
// (parser.Collect, the artifact's home holding the run's recorder) and
// returns its rows as timeline.jsonl writes them, through the function the
// timeline command writes each row with (timeline.Row.Withheld).
func parseWithheld(subject parser.Parser, artifact *input.Artifact, options parser.Options) ([]Row, error) {
	recorder := secrets.NewRecorder(withheldSecrets())
	if artifact.Home != nil {
		artifact.Home.Secrets = recorder
	}
	var rows []timeline.Row
	_, err := parser.Collect(subject, artifact, options, func(row timeline.Row) { rows = append(rows, row) })
	got := make([]Row, len(rows))
	for i, row := range rows {
		got[i] = replayRow(row.Withheld(recorder))
	}
	return got, err
}

// withSecretsWithheld returns the arguments after a recorded detect or
// timeline command with --no-secrets and --redaction-placeholder naming
// replayPlaceholder ahead of them, so the run withholds secrets as every
// recorded run did; any other command's arguments are returned as they
// are.
func withSecretsWithheld(command string, args []string) []string {
	if command != "detect" && command != "timeline" {
		return args
	}
	return slices.Concat([]string{"--no-secrets", "--redaction-placeholder", replayPlaceholder}, args)
}

// withheldDifference returns difference 230.
func withheldDifference() Difference {
	return Difference{Number: 230, Summary: "a withheld secret is written as the placeholder: at each site of " +
		"withheldForms, the recording's withheld form and the binary's form with the placeholder become one token " +
		"that keeps the rest of the row; where the recording cut a URL at its query, its row takes the URL " +
		"the case's input gives, its credentials as the placeholder (those the binary's rule finds, which must " +
		"include the replay's own minimum, a password and a key parameter), and the binary's row is compared with " +
		"it as written",
		Text: formsText(withheldForms(), recordedSide, caseRun{}), Binary: formsText(withheldForms(), binarySide,
			caseRun{}), RowText: withheldRowText(recordedSide, caseRun{}),
		RowBinary: withheldRowText(binarySide, caseRun{}),
		Within:    102}
}

// withheldRowsDifference returns difference 231.
func withheldRowsDifference() Difference {
	return Difference{Number: 231, Summary: "a secret the recording never printed and had no row for is " +
		"shown in a row of its own: a parse may add one row of each entry of withheldRows, exactly as " +
		"written there, with the placeholder where the secret was", Added: withheldRowAdded}
}

// withheldRow is a row the binary adds for a secret the recording never
// printed and gave no row for: the agent whose parser gives it, a name,
// and the row's exact text with replayPlaceholder in place of the secret.
type withheldRow struct {
	agent, name, text string
	row               addedRow
}

// label returns the name the run's log and its check of every entry use.
func (r withheldRow) label() string {
	return r.agent + " " + r.name
}

// withheldRows returns the rows difference 231 takes. A parser that comes
// to show a secret in a row of its own adds an entry, and a run of every
// case fails for an entry no case used.
func withheldRows() []withheldRow {
	return []withheldRow{
		{"agent-zero", "connector settings", "connector settings: AGENT_ZERO_HOST=http://localhost:50080" +
			" AGENT_ZERO_COMPUTER_USE_RESTORE_TOKEN=" + replayPlaceholder +
			" AGENT_ZERO_LAST_CONTEXT_HOST=https://a0.example.net AGENT_ZERO_LAST_CONTEXT_ID=RemCtx01" +
			" (from .agent-zero/.env, file mtime)", addedRow{time: fileTime}},
		{"letta", "settings API key", "stored credentials: env.LETTA_API_KEY=" + replayPlaceholder +
			" (from .letta/settings.json, file mtime)", addedRow{time: fileTime, line: 1}},
		{"shellgpt", "API key", "stored credentials: OPENAI_API_KEY=" + replayPlaceholder +
			" (from .config/shell_gpt/.sgptrc, file mtime)", addedRow{time: fileTime, line: 1}},
		{"crush", "provider API key", "stored credentials: providers.anthropic.api_key=" + replayPlaceholder +
			" (from .crush/crush.json, file mtime)", addedRow{time: fileTime, project: homePath, line: 1}},
		{"nanobot", "provider API key", "stored credentials: providers.openai.apiKey=" + replayPlaceholder +
			" (from .nanobot/config.json, file mtime)", addedRow{time: fileTime}},
		{"little-coder", "legacy config tokens", "stored credentials: gist_token=" + replayPlaceholder +
			" telegram_token=" + replayPlaceholder + " (from .little-coder/config.json, file mtime)", addedRow{time: fileTime, line: 1}},
		{"twinny", "inference provider API key", "model provider: p1 label=Ollama provider=ollama type=chat " +
			"model=qwen2.5-coder:7b apiKey=" + replayPlaceholder +
			" (from rjmacarthy.twinny twinny.inference-providers, file mtime)", addedRow{time: fileTime}},
		{"twinny", "active chat provider API key", "model provider: active chat p1 apiKey=" +
			replayPlaceholder + " (from rjmacarthy.twinny twinny.active-chat-provider, file mtime)", addedRow{time: fileTime}},
		{"tabby", "user auth token", "tabby user: alice@example.com (Alice) id=1 auth_token=" +
			replayPlaceholder + " (from users table, created_at)", addedRow{cell: tableCell{"users", "created_at", "1"}, line: 1}},
		{"tabby", "server record token", "tabby server: http://localhost:8080 token=" + replayPlaceholder +
			" (from state.vscdb TabbyML.vscode-tabby server.serverRecords, no times)", addedRow{}},
	}
}

// withheldRowAdded takes from got, for each of the agent's withheldRows,
// the first system row of the parsed file whose text is the entry's and
// that want does not hold, and returns the rest and the labels of the
// entries it took a row for, one per line, or "". A parse may add one row
// of each entry; a second stays and is compared.
func withheldRowAdded(parse parsedFile, want, got []Row) ([]Row, string) {
	var labels []string
	for _, entry := range withheldRows() {
		if entry.agent != parse.agent {
			continue
		}
		var taken bool
		if got, taken = takeRow(parse.matches(entry.text, entry.row, want), want, got); taken {
			labels = append(labels, entry.label())
		}
	}
	return got, strings.Join(labels, "\n")
}

// withheldRowLabels returns the labels of the entries of withheldRows that
// taken, the text withheldRowAdded returns, names.
func withheldRowLabels(taken string) []string {
	var labels []string
	for _, label := range strings.Split(taken, "\n") {
		if slices.ContainsFunc(withheldRows(), func(entry withheldRow) bool { return entry.label() == label }) {
			labels = append(labels, label)
		}
	}
	return labels
}

// withheldForm is one site where the recording's form of a secret differs
// from the binary's withheld form: whose output it is (scope, a parser's
// rows or a command's own output, and agent, the parser's agent or the
// command), a name, the lines it may appear on (nil for any), the recorded
// form and the binary's form with the placeholder, each a pattern that
// stops at the end of the secret's part so the rest is still compared, and
// the token both become. A site where the binary adds a part the recording
// never printed (built by addsForm) has the marker in its token.
type withheldForm struct {
	scope, agent, name string
	lines              *regexp.Regexp
	recorded, binary   *regexp.Regexp
	token              string
	// cause, for a command entry, is what a file of the case's input tree
	// must hold for the entry to apply (caseRun.applies).
	cause *regexp.Regexp
	// completed, when set, takes the place of token and binary: it gives,
	// for the submatches of a match of recorded, the text the binary is to
	// write there, from the case's input, and the binary's text is
	// compared as written.
	completed func(match []string, run caseRun) string
}

// addsForm returns a parser's entry for a site where the binary adds a part
// the recording never printed, at one place: the recorded row is first
// then second, the binary's row first, the part once, then second, and
// both become first, the entry's marker, then second. So a binary that
// leaves the part out, writes it twice, writes it elsewhere or writes
// another part still fails. Neither pattern may hold a capturing group.
func addsForm(agent, name, first, part, second string) withheldForm {
	form := withheldForm{scope: parserScope, agent: agent, name: name,
		recorded: regexp.MustCompile("(" + first + ")(" + second + ")"),
		binary:   regexp.MustCompile("(" + first + ")" + part + "(" + second + ")")}
	form.token = "${1}" + form.marker() + "${2}"
	return form
}

// The scopes of withheldForms: the replay whose cases each entry's rows
// come from, and which checks that some case used it.
const (
	parserScope  = "parser"
	commandScope = "command"
)

// label returns the name the run's log and its check of every entry use.
func (f withheldForm) label() string {
	return f.agent + " " + f.name
}

// rewrite returns a command's output text with the entry's form of one
// side as its token, and the count: a command entry's on its lines, a
// parser entry's on the lines of its agent's rows only (a timeline.jsonl
// line naming the agent).
func (f withheldForm) rewrite(from side, text string, run caseRun) (string, int) {
	lines := f.lines
	if f.scope == parserScope {
		lines = regexp.MustCompile(`(?m)^.*"agent": ?"` + regexp.QuoteMeta(f.agent) + `".*$`)
	}
	if lines == nil {
		return f.replace(from, text, run)
	}
	found := 0
	text = lines.ReplaceAllStringFunc(text, func(part string) string {
		part, count := f.replace(from, part, run)
		found += count
		return part
	})
	return text, found
}

// rowRewrite returns the text of a row of agent's parser with a parser
// entry's form of one side as its token, and the count; another agent's
// row, and every row for a command entry, is left as it is.
func (f withheldForm) rowRewrite(from side, agent, text string, run caseRun) (string, int) {
	if f.scope != parserScope || f.agent != agent {
		return text, 0
	}
	return f.replace(from, text, run)
}

// marker returns the text that stands, in the compared form, where the
// binary adds a part the recording never printed.
func (f withheldForm) marker() string {
	return "{added " + f.name + "}"
}

// replace returns text with every match of the entry's pattern of one side
// as its token, and the count; for an entry the input completes, the
// recorded side's matches as completed gives them, and the binary's side
// as it is.
func (f withheldForm) replace(from side, text string, run caseRun) (string, int) {
	if f.completed != nil {
		return f.complete(from, text, run)
	}
	pattern := f.recorded
	if from == binarySide {
		pattern = f.binary
		// The binary never writes the token's own text, so a text that
		// holds it is kept from comparing equal to the recording's compared
		// form.
		if own := f.tokenText(); own != "" {
			text = strings.ReplaceAll(text, own, "{written "+strings.TrimPrefix(own, "{"))
		}
	}
	return pattern.ReplaceAllString(text, f.token), len(pattern.FindAllStringIndex(text, -1))
}

// submatchReference is a reference to a submatch in a token.
var submatchReference = regexp.MustCompile(`\$\{[0-9]+\}`)

// tokenText returns the first part of the entry's token that is its own
// text rather than a submatch of the text it replaces, such as
// "{withheld token}" or an addsForm entry's marker, or "" for none.
func (f withheldForm) tokenText() string {
	for _, part := range submatchReference.Split(f.token, -1) {
		if part != "" {
			return part
		}
	}
	return ""
}

// complete returns the recorded side's text with each match of the
// entry's recorded pattern as completed gives it in run, and the count;
// the binary's side is returned as it is, with none.
func (f withheldForm) complete(from side, text string, run caseRun) (string, int) {
	if from == binarySide {
		return text, 0
	}
	var written strings.Builder
	end := 0
	found := f.recorded.FindAllStringSubmatchIndex(text, -1)
	for _, bounds := range found {
		match := make([]string, len(bounds)/2)
		for i := range match {
			if bounds[2*i] >= 0 {
				match[i] = text[bounds[2*i]:bounds[2*i+1]]
			}
		}
		written.WriteString(text[end:bounds[0]] + f.completed(match, run))
		end = bounds[1]
	}
	written.WriteString(text[end:])
	return written.String(), len(found)
}

// parserForm returns the entry of difference 230 for a parser whose reader
// error or token the binary withholds: the form of the recording and of the
// binary, and the token both are rewritten to.
func parserForm(agent, name, recorded, binary, token string) withheldForm {
	return withheldForm{scope: parserScope, agent: agent, name: name, recorded: regexp.MustCompile(recorded),
		binary: regexp.MustCompile(binary), token: token}
}

// withheldInPlaceForms returns the entries of difference 230 in which the binary
// writes the placeholder where the recording wrote the reader's detail, said
// that a token was present, or wrote a note that a line was not shown.
func withheldInPlaceForms() []withheldForm {
	pythonJSON := `(?:` + strings.Join(pythonJSONMessages(), "|") + `): line ([0-9]+) column ([0-9]+) \(char [0-9]+\)`
	goJSON := `invalid JSON at line ([0-9]+), column ([0-9]+): ` + replayPlaceholder
	position := "{withheld at line ${2}, column ${3}}"
	return []withheldForm{
		parserForm("tabby", "config.toml reader error",
			`(parser: config\.toml is not valid TOML \()(?:`+strings.Join(tomlMessages(), "|")+`)[^"\n]*? `+
				`\(at line ([0-9]+), column ([0-9]+)\)\)`,
			`(parser: config\.toml is not valid TOML \()invalid TOML at line ([0-9]+), column ([0-9]+): `+
				replayPlaceholder+`\)`,
			"${1}"+position+")"),
		// The binary gives an Agent Zero chat.json reader's error through
		// ReaderErrorText, the file holding the bot token and API keys.
		parserForm("agent-zero", "chat.json reader error",
			`(parser: chat\.json did not parse \()`+pythonJSON+`(\); log items recovered without history)`,
			`(parser: chat\.json did not parse \()`+goJSON+`(\); log items recovered without history)`,
			"${1}"+position+"${4}"),
		// The recording left out a .sgptrc line without "=", the file holding
		// the API key; the binary gives it through SecretText, withheld.
		parserForm("shellgpt", "settings line without an equals sign",
			`(parser: unparseable line [0-9]+ \(no "=" separator); not shown, the file holds the API key\)`,
			`(parser: unparseable line [0-9]+ \(no "=" separator)\): `+replayPlaceholder,
			"${1}{withheld line})"),
		parserForm("nanobot", "config.json reader error",
			`(parser: config\.json did not parse: )`+pythonJSON+`("|\n|\z)`,
			`(parser: config\.json did not parse: )`+goJSON+`("|\n|\z)`,
			"${1}"+position+"${4}"),
		parserForm("openclaw", "exec approvals reader error",
			`(exec_approvals_config raw_json unparseable \()`+pythonJSON+`; text withheld, it holds the approvals `+
				`socket token\)`,
			`(exec_approvals_config raw_json unparseable \()`+goJSON+`\)`,
			"${1}"+position+")"),
		// The recording said only that an account's token was present; the
		// binary gives the token, withheld.
		parserForm("copilot-desktop", "account token",
			`(account: [^"\n]* )token_present=true("|\n|\z)`,
			`(account: [^"\n]* )token=`+replayPlaceholder+`("|\n|\z)`,
			"${1}{withheld token}${2}"),
	}
}

// addedPartForms returns the entries of difference 230 in which the binary adds
// a part, a token or a reader's error, that the recording never printed.
func addedPartForms() []withheldForm {
	return []withheldForm{
		// The recording names a crush.json that is not JSON without the
		// reader's error; the binary adds it, withheld.
		addsForm("crush", "crush.json reader error", `parser: [^"\n]*crush\.json is not a JSON object`,
			` \(invalid JSON at line [0-9]+, column [0-9]+: `+replayPlaceholder+`\)`, `"|\n|\z`),
		// The recording gave a desktop connection without the token and
		// headers stored in plain text beside it; the binary adds them. The
		// recorded files hold a token and no headers.
		addsForm("hermes", "connections.json token", `remote backend: remote [^"\n]*?`,
			` token=`+replayPlaceholder, ` \(from connections\.json, file mtime\)`),
		// The recording gave a Telegram chat's relay without the bot's token
		// and webhook secret; the binary adds them. The recorded files hold
		// a token and no webhook secret.
		addsForm("agent-zero", "telegram bot token", `session relayed: telegram bot [^"\n]*?`,
			` token=`+replayPlaceholder, ` \(from chat\.json data, file mtime\)`),
		// The recording gave a chat override's slots without the API key a
		// slot holds; the binary adds it after the slot. The recorded files
		// hold a key for the chat slot alone.
		addsForm("agent-zero", "model override chat API key", `model: requested chat=[^ "\n]*`,
			` chat\.api_key=`+replayPlaceholder, ` [^"\n]*?\(chat override, as of file mtime\)`),
		// The recording gave a retried upload without its bearer token; the
		// binary adds it after the URL.
		addsForm("roo-code", "retry queue bearer token", `cloud upload queued: [^"\n]*? url=[^ "\n]*`,
			` Authorization=`+replayPlaceholder, ` [^"\n]*?\(Roo Code Cloud retry queue, `),
		// The recording gave an integration without its access token.
		addsForm("tabby", "integration access token", `source host: [^"\n]* api_base=[^ "\n]*`,
			` access_token=`+replayPlaceholder,
			` \(account not recorded; from integrations table, created_at\)`),
		// The recording gave a configured model without its API key, which
		// the binary writes after the kind and the endpoint.
		addsForm("tabby", "config.toml API key",
			`model configured: [^"\n]* via http(?: kind=[^ "\n]*)?(?: endpoint=[^ "\n]*)?`,
			` api_key=`+replayPlaceholder, `(?: supported_models=[^ "\n]*)? \(from config\.toml \[model\.`),
		// The recording gave a session's share without its secret.
		addsForm("opencode", "share secret", `session synced: share [^"\n]* url [^ "\n]*`,
			` secret=`+replayPlaceholder, ` \(from [^"\n]* session_share, time_created\)`),
	}
}

// commandAndURLForms returns the entries of difference 230 that read a command's
// output or a URL: a pi row that equals a recorded key, a network request's URL,
// and the user information of a repository's remote URLs.
func commandAndURLForms() []withheldForm {
	schemeText := `[A-Za-z][A-Za-z0-9+.-]*://`
	scheme := "(" + schemeText + ")"
	return []withheldForm{
		// The binary withholds a URL credential of 8 bytes or more wherever
		// the output holds it, so in a run over a tree holding a Gemini CLI
		// request whose URL gives key=redacted, a pi thinking row the agent
		// wrote as [redacted] reads [WITHHELD_BY_REPLAY]. The entry applies only in a
		// case whose input tree holds such a request.
		{scope: commandScope, agent: "pi", name: "thinking row equal to a recorded key",
			lines:    regexp.MustCompile(`(?m)^.*"agent": ?"pi".*$`),
			recorded: regexp.MustCompile(`("text": ?")\[redacted\](")`),
			binary:   regexp.MustCompile(`("text": ?")\[` + replayPlaceholder + `\](")`),
			token:    "${1}{a key's text}${2}", cause: regexp.MustCompile(`https?://[^\s"?]*\?(?:[^\s"#]*&)?key=redacted\b`)},
		// The recording cut a network request's URL at its query, which can
		// carry an API key; the binary gives the whole URL as the request's
		// record in the input holds it, its credentials withheld
		// (requestURL), a password and a key parameter whatever the binary's
		// table says. An error row has a colon after the URL.
		{scope: parserScope, agent: "gemini-cli", name: "network request URL",
			recorded: regexp.MustCompile(`(network (?:request|response|error): [A-Z]+ )(https?://[^ ?#"\n]*?)` +
				`(:? (?:[^"\n]*? )?\(request ([^ )"\n]*)\))`),
			completed: func(match []string, run caseRun) string {
				return match[1] + run.requestURL(match[4], match[2]) + match[3]
			}},
		// The recording printed a repository's remote URLs as written; the
		// binary run with --no-secrets withholds their user information, on
		// detect's remotes: line and in the urls and pushurls arrays of
		// detect --json and inventory.json, one URL per line, as
		// remoteUserInfo describes, so ssh://git@host is printed as written.
		// A scheme takes letters of either case, as the binary's URL grammar
		// does. Only user information is covered: a remote whose query holds
		// a credential parameter the binary withholds fails.
		{scope: commandScope, agent: "detect and timeline", name: ".git/config remote URL",
			lines:    regexp.MustCompile(`(?m)^(?:  remotes: [^\n]*|[ \t]*"` + schemeText + `[^"\n]*",?)$`),
			recorded: regexp.MustCompile(scheme + remoteUserInfo() + `@`),
			binary:   regexp.MustCompile(scheme + replayPlaceholder + `@`), token: "${1}{withheld}@"},
	}
}

// withheldForms returns the entries of difference 230. A parser that comes
// to withhold a secret the recording gave in another form adds an entry,
// and a run of every case fails for an entry no case of its scope used.
// Each of the binary's forms holds the placeholder where the secret was,
// so a binary that shows the secret, or prints the recording's form,
// still fails. The Gemini CLI request URL is completed instead: the
// recorded row takes the URL the case's input gives, its credentials as
// the placeholder, so a binary that drops, shows or repeats a part of it
// fails.
func withheldForms() []withheldForm {
	return slices.Concat(withheldInPlaceForms(), addedPartForms(), commandAndURLForms())
}

// remoteUserInfo returns the pattern of the user information of a URL that
// the binary withholds, up to its last @ in the authority, which ends at
// white space, a double quote, an angle bracket, a backtick, /, ?, # or a
// backslash: with a colon and more than the colon alone, or without one and
// at least secrets.EverywhereLength bytes long.
func remoteUserInfo() string {
	authority := "[^\\s\"<>`/?#\\\\]"
	name := "[^\\s\"<>`/?#\\\\:]"
	return "(?:" + authority + "+:" + authority + "*|:" + authority + "+|" + name + "{" +
		strconv.Itoa(secrets.EverywhereLength) + ",})"
}

// withheldIn returns table with difference 230's rewrites of texts and of
// rows those of run.
func withheldIn(table []Difference, run caseRun) []Difference {
	table = slices.Clone(table)
	for i := range table {
		if table[i].Number == withheldDifference().Number {
			table[i].Text, table[i].Binary = formsText(withheldForms(), recordedSide, run),
				formsText(withheldForms(), binarySide, run)
			table[i].RowText, table[i].RowBinary = withheldRowText(recordedSide, run),
				withheldRowText(binarySide, run)
		}
	}
	return table
}

// formsText returns the rewrite of a command case's texts for one side by
// the entries of forms that apply in run: each entry's form of that side as
// its token, and the count.
func formsText(forms []withheldForm, from side, run caseRun) func(text string) (string, int) {
	return func(text string) (string, int) {
		total := 0
		for _, entry := range forms {
			if !run.applies(entry) {
				continue
			}
			var found int
			text, found = entry.rewrite(from, text, run)
			total += found
		}
		return text, total
	}
}

// withheldRowText returns the rewrite of difference 230 of a row's text for
// one side in run: each parser entry of the row's agent's form of that side
// as its token, and the count.
func withheldRowText(from side, run caseRun) func(agent, text string) (string, int) {
	return func(agent, text string) (string, int) {
		return rowRewriteAll(withheldForms(), from, agent, text, run)
	}
}

// rowRewriteAll returns text rewritten by each form's rowRewrite in run, and
// the count.
func rowRewriteAll(forms []withheldForm, from side, agent, text string, run caseRun) (string, int) {
	total := 0
	for _, entry := range forms {
		var found int
		text, found = entry.rowRewrite(from, agent, text, run)
		total += found
	}
	return text, total
}

// rowEntriesIn returns the labels of the forms whose recorded form shows in
// a row of rows of the form's own agent.
func rowEntriesIn(forms []withheldForm, rows []Row) []string {
	var labels []string
	for _, entry := range forms {
		for _, row := range rows {
			if _, found := entry.rowRewrite(recordedSide, string(row.Agent), string(row.Text), caseRun{}); found > 0 {
				labels = append(labels, entry.label())
				break
			}
		}
	}
	return labels
}

// withheldEntriesIn returns the labels of the entries of scope that apply
// in run and whose recorded form shows in text.
func withheldEntriesIn(scope, text string, run caseRun) []string {
	var labels []string
	for _, entry := range withheldForms() {
		if !run.applies(entry) {
			continue
		}
		if _, found := entry.rewrite(recordedSide, text, run); found > 0 && entry.scope == scope {
			labels = append(labels, entry.label())
		}
	}
	return labels
}

// checkEveryWithheldFormUsed fails the test for each entry of
// withheldForms of scope that no case of a whole run of that scope's
// replay needed.
func checkEveryWithheldFormUsed(t testing.TB, scope string, used map[string]int, whole bool) {
	t.Helper()
	if !whole {
		return
	}
	for _, entry := range withheldForms() {
		if entry.scope != scope {
			continue
		}
		if used[entry.label()] == 0 {
			t.Errorf("difference 230's entry %s is used by no recorded case: remove it", entry.label())
			continue
		}
		t.Logf("difference 230's entry %s used by %d cases", entry.label(), used[entry.label()])
	}
	if scope != parserScope {
		return
	}
	for _, entry := range withheldRows() {
		if used[entry.label()] == 0 {
			t.Errorf("difference 231's entry %s is used by no recorded case: remove it", entry.label())
			continue
		}
		t.Logf("difference 231's entry %s used by %d cases", entry.label(), used[entry.label()])
	}
}

func TestAWithheldSecretsRowIsTheOnlyRowItLetsAParseAdd(t *testing.T) {
	const file, rel = "/h/.agent-zero/.env", ".agent-zero/.env"
	parse := parsedFile{agent: "agent-zero", file: file, rel: rel}
	origin := Row{TurnType: "system", SourceFile: file, SessionID: "RemCtx01",
		Text: "session origin: remote agent-zero instance https://a0.example.net context RemCtx01 " +
			"(from .agent-zero/.env, file mtime)"}
	settings := Row{TurnType: "system", Agent: "agent-zero", SourceFile: file, Text: Text(withheldRows()[0].text)}
	shown := settings
	shown.Text = Text(strings.Replace(string(settings.Text), replayPlaceholder, "fixture-restore-token", 1))
	elsewhere := settings
	elsewhere.SourceFile = "/h/other.env"
	notSystem := settings
	notSystem.TurnType = "user"
	changed := origin
	changed.Text = Text(strings.Replace(string(origin.Text), "RemCtx01", "RemCtx02", 1))
	dated, inSession, atLine, ofMallory := settings, settings, settings, settings
	dated.TimestampUTC = "1999-01-01T00:00:00.000Z"
	inSession.SessionID = "WRONG-SESSION"
	atLine.SourceLine = 99
	ofMallory.User = "mallory"
	defaultPlaceholder := settings
	defaultPlaceholder.Text = Text(strings.ReplaceAll(string(settings.Text), replayPlaceholder,
		secrets.DefaultPlaceholder))
	tests := []struct {
		name      string
		parse     parsedFile
		want, got []Row
		agree     bool
	}{
		{"an added withheld settings row passes", parse, []Row{origin}, []Row{settings, origin}, true},
		{"the settings row showing a secret fails", parse, []Row{origin}, []Row{shown, origin}, false},
		{"the row added twice fails", parse, []Row{origin}, []Row{settings, settings, origin}, false},
		{"the row of another file fails", parse, []Row{origin}, []Row{elsewhere, origin}, false},
		{"the row as another turn fails", parse, []Row{origin}, []Row{notSystem, origin}, false},
		{"the row added by another agent's parse fails", parsedFile{agent: "hermes", file: file, rel: rel}, []Row{origin},
			[]Row{settings, origin}, false},
		{"a changed row beside it fails", parse, []Row{origin}, []Row{settings, changed}, false},
		{"the row at another time fails", parse, []Row{origin}, []Row{dated, origin}, false},
		{"the row in a session fails", parse, []Row{origin}, []Row{inSession, origin}, false},
		{"the row at another line fails", parse, []Row{origin}, []Row{atLine, origin}, false},
		{"the row of another user fails", parse, []Row{origin}, []Row{ofMallory, origin}, false},
		{"the row with the default placeholder a parser wrote fails", parse, []Row{origin},
			[]Row{defaultPlaceholder, origin}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _, _ := addedRows(test.parse, test.want, test.got)
			if agree, _ := rowsAgree(t, test.want, got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestAParseMayAddOneRowOfEachWithheldRowEntry(t *testing.T) {
	const file, rel = "/h/User/globalStorage/state.vscdb", "User/globalStorage/state.vscdb"
	parse := parsedFile{agent: "twinny", file: file, rel: rel}
	var rows []Row
	for _, entry := range withheldRows() {
		if entry.agent == "twinny" {
			rows = append(rows, Row{TurnType: "system", Agent: "twinny", SourceFile: file, Text: Text(entry.text)})
		}
	}
	if len(rows) != 2 {
		t.Fatalf("%d entries for twinny, want 2", len(rows))
	}
	tests := []struct {
		name  string
		got   []Row
		agree bool
	}{
		{"a row of each entry passes", rows, true},
		{"one row of one entry passes", rows[:1], true},
		{"a row of one entry twice fails", []Row{rows[0], rows[0]}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _, _ := addedRows(parse, nil, test.got)
			if agree, _ := rowsAgree(t, nil, got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

// withheldCase is one case of the comparison of a withheld secret: the text
// recorded, the text the binary wrote, and whether the comparison agrees.
type withheldCase struct {
	name, want, got string
	agree           bool
}

// remoteCases returns the cases of the remote URLs of a repository, where the
// binary withholds the user information of a URL.
func remoteCases() []withheldCase {
	remoteRecorded := "  remotes: origin git@h:o/r.git (push https://alice:TOKEN@github.com/o/r.git)\n"
	remoteBinary := strings.Replace(remoteRecorded, "alice:TOKEN", replayPlaceholder, 1)
	jsonRemote := "          \"urls\": [\n            \"https://alice:TOKEN@github.com/o/r.git\"\n"
	upperRecorded := strings.Replace(remoteRecorded, "https://", "HTTPS://", 1)
	upperJSON := strings.Replace(jsonRemote, "https://", "HTTPS://", 1)
	queryRecorded := "  remotes: origin https://h/o/r.git?access_token=abcdefghij\n"
	return []withheldCase{
		{"a remote in its placeholder form passes", remoteRecorded, remoteBinary, true},
		{"a remote showing its credentials fails", remoteRecorded, remoteRecorded, false},
		{"a remote of another host fails", remoteRecorded, strings.Replace(remoteBinary, "github.com", "gitlab.com", 1),
			false},
		{"a remote of another name fails", remoteRecorded, strings.Replace(remoteBinary, "origin", "upstream", 1), false},
		{"a remote URL in JSON in its placeholder form passes", jsonRemote,
			strings.Replace(jsonRemote, "alice:TOKEN", replayPlaceholder, 1), true},
		{"a URL with credentials in a row's text is not this entry's", "{\"text\":\"https://alice:TOKEN@h/\"}\n",
			"{\"text\":\"https://" + replayPlaceholder + "@h/\"}\n", false},
		{"a remote written as the compared form's text fails", remoteRecorded,
			strings.Replace(remoteRecorded, "alice:TOKEN@", "{withheld}@", 1), false},
		{"a remote of an upper-case scheme in its placeholder form passes", upperRecorded,
			strings.Replace(upperRecorded, "alice:TOKEN", replayPlaceholder, 1), true},
		{"a remote URL in JSON of an upper-case scheme in its placeholder form passes", upperJSON,
			strings.Replace(upperJSON, "alice:TOKEN", replayPlaceholder, 1), true},
		{"a remote of an upper-case scheme showing its credentials fails", upperRecorded, upperRecorded, false},
		// The entry covers user information alone: a credential parameter
		// withheld in a remote's query is a difference no entry takes.
		{"a remote whose query parameter is withheld fails", queryRecorded,
			strings.Replace(queryRecorded, "abcdefghij", replayPlaceholder, 1), false},
	}
}

// tomlJSONAndChatErrorCases returns the cases of the reader errors of a TOML
// config, a JSON config and a chat.json, each withheld at its position.
func tomlJSONAndChatErrorCases() []withheldCase {
	tomlRecorded := "parser: config.toml is not valid TOML (Invalid value (at line 2, column 11))"
	tomlBinary := "parser: config.toml is not valid TOML (invalid TOML at line 2, column 11: " + replayPlaceholder + ")"
	jsonRecorded := "parser: config.json did not parse: Expecting value: line 1 column 37 (char 36)"
	jsonBinary := "parser: config.json did not parse: invalid JSON at line 1, column 37: " + replayPlaceholder
	chatRecorded := "parser: chat.json did not parse (Expecting value: line 1 column 3317 (char 3316)); " +
		"log items recovered without history"
	chatBinary := "parser: chat.json did not parse (invalid JSON at line 1, column 3317: " + replayPlaceholder + "); " +
		"log items recovered without history"
	return []withheldCase{
		{"a TOML error in its placeholder form passes", tomlRecorded, tomlBinary, true},
		{"a TOML error showing the reader's detail fails", tomlRecorded,
			strings.Replace(tomlBinary, replayPlaceholder, `expected value but found "sk" instead`, 1), false},
		{"a TOML error at another column fails", tomlRecorded, strings.Replace(tomlBinary, "11", "12", 1), false},
		{"a TOML error of another file fails", tomlRecorded, strings.Replace(tomlBinary, "config", "other", 1), false},
		{"a JSON error in its placeholder form passes", jsonRecorded, jsonBinary, true},
		{"a JSON error showing the reader's detail fails", jsonRecorded,
			strings.Replace(jsonBinary, replayPlaceholder, "invalid character 's' looking for beginning of value", 1), false},
		{"a JSON error on another line fails", jsonRecorded, strings.Replace(jsonBinary, "line 1", "line 2", 1), false},
		{"a JSON error in another placeholder fails", jsonRecorded, strings.Replace(jsonBinary, replayPlaceholder, "x", 1),
			false},
		{"the recording's wording printed by the binary fails", jsonRecorded, jsonRecorded, false},
		{"a chat.json error in its placeholder form passes", chatRecorded, chatBinary, true},
		{"a chat.json error showing the reader's detail fails", chatRecorded,
			strings.Replace(chatBinary, replayPlaceholder, "invalid character 's'", 1), false},
		{"a JSON error written as the compared form's text fails", jsonRecorded,
			"parser: config.json did not parse: {withheld at line 1, column 37}", false},
	}
}

// crushExecAndSettingsLineCases returns the cases of the crush.json and exec approvals
// reader errors and of a settings line the binary withholds.
func crushExecAndSettingsLineCases() []withheldCase {
	crushRecorded := "parser: .config/crush/crush.json is not a JSON object"
	crushBinary := crushRecorded + " (invalid JSON at line 1, column 38: " + replayPlaceholder + ")"
	execRecorded := "parser: exec_approvals_config raw_json unparseable (Expecting value: line 1 column 22 " +
		"(char 21); text withheld, it holds the approvals socket token)"
	execBinary := "parser: exec_approvals_config raw_json unparseable (invalid JSON at line 1, column 22: " + replayPlaceholder + ")"
	lineRecorded := `parser: unparseable line 1 (no "=" separator; not shown, the file holds the API key)`
	lineBinary := `parser: unparseable line 1 (no "=" separator): ` + replayPlaceholder
	return []withheldCase{
		{"a crush.json error in its placeholder form passes", crushRecorded, crushBinary, true},
		{"a crush.json error showing the reader's detail fails", crushRecorded,
			strings.Replace(crushBinary, replayPlaceholder, "invalid character 's'", 1), false},
		{"a crush.json error naming another file fails", crushRecorded,
			strings.Replace(crushBinary, ".config", ".local", 1), false},
		{"an exec approvals error in its placeholder form passes", execRecorded, execBinary, true},
		{"an exec approvals error showing the reader's detail fails", execRecorded,
			strings.Replace(execBinary, replayPlaceholder, "invalid character 's'", 1), false},
		{"an exec approvals error at another column fails", execRecorded,
			strings.Replace(execBinary, "22", "23", 1), false},
		{"an exec approvals error of another table fails", execRecorded,
			strings.Replace(execBinary, "exec_approvals_config", "exec_approvals", 1), false},
		{"a settings line in its placeholder form passes", lineRecorded, lineBinary, true},
		{"a settings line shown fails", lineRecorded, strings.Replace(lineBinary, replayPlaceholder, "sk-1", 1), false},
		{"a settings line of another number fails", lineRecorded, strings.Replace(lineBinary, "line 1", "line 2", 1),
			false},
		{"a crush.json error without the reader's error the binary adds fails", crushRecorded, crushRecorded,
			false},
		{"a reader's error a parser wrote as the default placeholder fails", crushRecorded,
			strings.Replace(crushBinary, replayPlaceholder, secrets.DefaultPlaceholder, 1), false},
	}
}

// accountAndBotTokenCases returns the cases of an account token and of a bot's
// token, which the binary writes as the placeholder.
func accountAndBotTokenCases() []withheldCase {
	accountRecorded := "account: ana host=github.com kind=github default=1 token_present=true"
	accountBinary := "account: ana host=github.com kind=github default=1 token=" + replayPlaceholder
	accountAbsent := "account: ana host=github.com kind=github default=1 token_present=false"
	telegramRecorded := "session relayed: telegram bot opsbot chat 1 user 2 @ana (from chat.json data, file mtime)"
	telegramBinary := strings.Replace(telegramRecorded, " (from", " token="+replayPlaceholder+" (from", 1)
	return []withheldCase{
		{"an account token in its placeholder form passes", accountRecorded, accountBinary, true},
		{"an account token shown fails", accountRecorded,
			strings.Replace(accountBinary, replayPlaceholder, "FIXTURE-TOKEN", 1), false},
		{"an account token in the recording's form fails", accountRecorded, accountRecorded, false},
		{"an account token of another login fails", accountRecorded, strings.Replace(accountBinary, "ana", "bo", 1),
			false},
		{"an absent account token passes as written", accountAbsent, accountAbsent, true},
		{"a bot token in its placeholder form passes", telegramRecorded, telegramBinary, true},
		{"a bot token shown fails", telegramRecorded, strings.Replace(telegramBinary, replayPlaceholder, "123:AA", 1), false},
		{"a bot of another chat fails", telegramRecorded, strings.Replace(telegramBinary, "chat 1", "chat 9", 1),
			false},
		{"an absent account token said to be present fails", accountAbsent, accountBinary, false},
		{"a bot without the token the binary adds fails", telegramRecorded, telegramRecorded, false},
		{"a bot's webhook secret in place of its token fails", telegramRecorded,
			strings.Replace(telegramBinary, " token=", " webhook_secret=", 1), false},
		{"an account token written as the compared form's text fails", accountRecorded,
			strings.Replace(accountRecorded, "token_present=true", "{withheld token}", 1), false},
		{"an account token a parser wrote as the default placeholder fails", accountRecorded,
			strings.Replace(accountBinary, replayPlaceholder, secrets.DefaultPlaceholder, 1), false},
	}
}

// connectionAndSlotKeyCases returns the cases of a connection's token and of a
// chat slot's API key, which the binary adds.
func connectionAndSlotKeyCases() []withheldCase {
	connectionRecorded := "remote backend: remote Homelab https://hermes.example.net (from connections.json, " +
		"file mtime)"
	connectionBinary := strings.Replace(connectionRecorded, " (from", " token="+replayPlaceholder+" (from", 1)
	overrideRecorded := "model: requested chat=openrouter/m utility=ollama/q (chat override, as of file mtime)"
	overrideBinary := strings.Replace(overrideRecorded, " utility", " chat.api_key="+replayPlaceholder+" utility", 1)
	return []withheldCase{
		{"a slot's API key in its placeholder form passes", overrideRecorded, overrideBinary, true},
		{"a slot's API key shown fails", overrideRecorded, strings.Replace(overrideBinary, replayPlaceholder, "sk-1", 1),
			false},
		{"a slot of another model fails", overrideRecorded, strings.Replace(overrideBinary, "ollama/q", "ollama/r", 1),
			false},
		{"a connection's token in its placeholder form passes", connectionRecorded, connectionBinary, true},
		{"a connection's token shown fails", connectionRecorded,
			strings.Replace(connectionBinary, replayPlaceholder, "not-real", 1), false},
		{"a connection of another URL fails", connectionRecorded,
			strings.Replace(connectionBinary, "hermes.example.net", "other.example", 1), false},
		{"a connection's token in another placeholder fails", connectionRecorded,
			strings.Replace(connectionBinary, replayPlaceholder, "x", 1), false},
		{"a connection without the token the binary adds fails", connectionRecorded, connectionRecorded, false},
		{"a slot without the API key the binary adds fails", overrideRecorded, overrideRecorded, false},
		{"a connection's token written twice fails", connectionRecorded,
			strings.Replace(connectionBinary, " (from", " token="+replayPlaceholder+" (from", 1), false},
		{"a connection's headers in place of its token fail", connectionRecorded,
			strings.Replace(connectionBinary, " token=", " headers=", 1), false},
		{"a connection's headers twice fail", connectionRecorded,
			strings.Replace(connectionBinary, " token="+replayPlaceholder, " headers="+replayPlaceholder+" headers="+replayPlaceholder, 1), false},
		{"a slot's API key after another slot fails", overrideRecorded,
			strings.Replace(overrideRecorded, " (chat", " utility.api_key="+replayPlaceholder+" (chat", 1), false},
		{"a token a parser wrote as the default placeholder fails", connectionRecorded,
			strings.Replace(connectionBinary, replayPlaceholder, secrets.DefaultPlaceholder, 1), false},
	}
}

// retryQueueCases returns the cases of a retried upload's token, which the
// binary adds.
func retryQueueCases() []withheldCase {
	retryRecorded := "cloud upload queued: Task Message task=t type=telemetry retries=2 " +
		"url=https://app.example/api/events lastError=fetch failed (Roo Code Cloud retry queue, from f " +
		"roo.retryQueue, request timestamp)"
	retryBinary := strings.Replace(retryRecorded, " lastError", " Authorization="+replayPlaceholder+" lastError", 1)
	return []withheldCase{
		{"a retried upload's token in its placeholder form passes", retryRecorded, retryBinary, true},
		{"a retried upload's token shown fails", retryRecorded,
			strings.Replace(retryBinary, replayPlaceholder, "Bearer eyJ", 1), false},
		{"a retried upload of another URL fails", retryRecorded,
			strings.Replace(retryBinary, "api/events", "api/other", 1), false},
		{"a retried upload of another error fails", retryRecorded,
			strings.Replace(retryBinary, "fetch failed", "timeout", 1), false},
		{"a retried upload without the token the binary adds fails", retryRecorded, retryRecorded, false},
		{"a retried upload's token written twice fails", retryRecorded,
			strings.Replace(retryBinary, " lastError", " Authorization="+replayPlaceholder+" lastError", 1), false},
		{"a retried upload's token after the error fails", retryRecorded,
			strings.Replace(retryRecorded, " (Roo Code", " Authorization="+replayPlaceholder+" (Roo Code", 1), false},
		{"a retried upload holding the compared form's marker fails", retryRecorded,
			strings.Replace(retryRecorded, " lastError", "{added retry queue bearer token} lastError", 1), false},
	}
}

// integrationModelAndShareCases returns the cases of an integration's token, a model's
// API key and a share's secret, which the binary adds.
func integrationModelAndShareCases() []withheldCase {
	integrationRecorded := "source host: github.com integration acme kind=github api_base=- " +
		"(account not recorded; from integrations table, created_at)"
	integrationBinary := strings.Replace(integrationRecorded, " (account", " access_token="+replayPlaceholder+" (account", 1)
	modelRecorded := "model configured: chat m via http kind=openai/chat endpoint=https://h/v1 " +
		"supported_models=m (from config.toml [model.chat.http], file mtime)"
	modelBinary := strings.Replace(modelRecorded, " supported", " api_key="+replayPlaceholder+" supported", 1)
	shareRecorded := "session synced: share SHARE05 url https://opncd.ai/share/SHARE05 " +
		"(from opencode-beta.db session_share, time_created)"
	shareBinary := strings.Replace(shareRecorded, " (from", " secret="+replayPlaceholder+" (from", 1)
	return []withheldCase{
		{"a share's secret in its placeholder form passes", shareRecorded, shareBinary, true},
		{"a share's secret shown fails", shareRecorded, strings.Replace(shareBinary, replayPlaceholder, "sec", 1), false},
		{"a share of another id fails", shareRecorded, strings.Replace(shareBinary, "SHARE05 url", "SHARE06 url", 1),
			false},
		{"a model's API key in its placeholder form passes", modelRecorded, modelBinary, true},
		{"a model's API key shown fails", modelRecorded, strings.Replace(modelBinary, replayPlaceholder, "sk-1", 1), false},
		{"a model of another endpoint fails", modelRecorded, strings.Replace(modelBinary, "h/v1", "g/v1", 1), false},
		{"an integration's token in its placeholder form passes", integrationRecorded, integrationBinary, true},
		{"an integration's token shown fails", integrationRecorded,
			strings.Replace(integrationBinary, replayPlaceholder, "ghp_x", 1), false},
		{"an integration of another kind fails", integrationRecorded,
			strings.Replace(integrationBinary, "kind=github", "kind=gitlab", 1), false},
		{"an integration without the token the binary adds fails", integrationRecorded, integrationRecorded, false},
		{"a model without the API key the binary adds fails", modelRecorded, modelRecorded, false},
		{"a share without the secret the binary adds fails", shareRecorded, shareRecorded, false},
		{"a model's API key after its supported models fails", modelRecorded,
			strings.Replace(modelRecorded, " (from", " api_key="+replayPlaceholder+" (from", 1), false},
		{"a share's secret before its URL fails", shareRecorded,
			strings.Replace(shareRecorded, " url", " secret="+replayPlaceholder+" url", 1), false},
	}
}

func TestAWithheldSecretIsComparedAsItsPlaceholderAndNothingElse(t *testing.T) {
	tests := slices.Concat(remoteCases(), tomlJSONAndChatErrorCases(), crushExecAndSettingsLineCases(),
		accountAndBotTokenCases(), connectionAndSlotKeyCases(), retryQueueCases(),
		integrationModelAndShareCases())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if agree := withheldAgree(t, recordedAgent(test.want), test.want, test.got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestARemoteURLIsComparedWithItsUserInformationWithheldOnlyWhereTheRuleWithholdsIt(t *testing.T) {
	tests := []struct {
		name, url, userInfo string
		withheld            bool
	}{
		{"a user name of three bytes", "ssh://git@github.com/o/r.git", "git", false},
		{"a user name of seven bytes", "https://abcdefg@github.com/o/r.git", "abcdefg", false},
		{"a user name of eight bytes", "https://abcdefgh@github.com/o/r.git", "abcdefgh", true},
		{"a user name and a password", "https://alice:TOKEN@github.com/o/r.git", "alice:TOKEN", true},
		{"a password alone", "https://:pw@github.com/o/r.git", ":pw", true},
		{"a user name and an empty password", "https://user:@github.com/o/r.git", "user:", true},
		{"the colon alone", "https://:@github.com/o/r.git", ":", false},
		{"user information up to the last @", "https://a@b:c@github.com/o/r.git", "a@b:c", true},
		{"a short user name before a port", "ssh://git@github.com:22/o/r.git", "git", false},
		{"an upper-case scheme", "HTTPS://alice:TOKEN@github.com/o/r.git", "alice:TOKEN", true},
		{"a scheme of mixed case", "Git+SSH://alice:TOKEN@github.com/o/r.git", "alice:TOKEN", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, recorded := range []string{"  remotes: origin " + test.url + "\n",
				"          \"urls\": [\n            \"" + test.url + "\"\n"} {
				withheld := strings.Replace(recorded, test.userInfo+"@", replayPlaceholder+"@", 1)
				if agree := withheldAgree(t, "", recorded, recorded); agree == test.withheld {
					t.Errorf("%q as written: agree %v, want %v", recorded, agree, !test.withheld)
				}
				if agree := withheldAgree(t, "", recorded, withheld); agree != test.withheld {
					t.Errorf("%q withheld: agree %v, want %v", recorded, agree, test.withheld)
				}
				recorder := secrets.NewRecorder(withheldSecrets())
				recorder.Collect(recorded)
				if binary := recorder.Text(recorded); !withheldAgree(t, "", recorded, binary) {
					t.Errorf("the binary's own text %q fails", binary)
				}
			}
		})
	}
}

func TestARemoteWhoseQueryHoldsACredentialFailsAsTheBinaryWritesIt(t *testing.T) {
	for _, recorded := range []string{"  remotes: origin https://h/o/r.git?access_token=abcdefghij\n",
		"          \"urls\": [\n            \"https://h/o/r.git?access_token=abcdefghij\"\n"} {
		recorder := secrets.NewRecorder(withheldSecrets())
		recorder.Collect(recorded)
		binary := recorder.Text(recorded)
		if binary == recorded {
			t.Fatalf("the binary does not withhold the parameter of %q", recorded)
		}
		if withheldAgree(t, "", recorded, binary) {
			t.Errorf("the binary's own text %q passes, which the remote URL entry does not cover", binary)
		}
	}
}

// recordedAgent returns the agent of the first parser entry of difference
// 230 whose recorded form shows in text, or "" for none.
func recordedAgent(text string) string {
	for _, entry := range withheldForms() {
		if _, found := entry.rowRewrite(recordedSide, entry.agent, text, caseRun{}); found > 0 {
			return entry.agent
		}
	}
	return ""
}

// withheldAgree reports whether a recorded and a binary text agree under
// differences 102 and 230: as a row of agent's parser, or as a command's
// output for no agent, of a case whose input records sampleRequests.
func withheldAgree(t *testing.T, agent, want, got string) bool {
	t.Helper()
	return withheldAgreeIn(t, caseRun{requests: sampleRequests()}, agent, want, got)
}

// withheldAgreeIn is withheldAgree for a case of run.
func withheldAgreeIn(t *testing.T, run caseRun, agent, want, got string) bool {
	t.Helper()
	reporter := &recordingTB{TB: t}
	table := withheldIn(Differences(), run)
	if agent == "" {
		normalWant, normalGot := normaliseTextsOf(reporter, table, "n", []int{102, 230}, want, got)
		return normalWant == normalGot
	}
	normalWant, normalGot := normaliseRowsOf(reporter, table, "n", []int{102, 230},
		[]Row{{Agent: Text(agent), Text: Text(want)}}, []Row{{Agent: Text(agent), Text: Text(got)}})
	return normalWant[0].Text == normalGot[0].Text
}

func TestAWithheldFormAppliesOnlyToItsAgentsRows(t *testing.T) {
	recorded := "remote backend: remote Homelab https://hermes.example.net (from connections.json, file mtime)"
	binary := strings.Replace(recorded, " (from", " token="+replayPlaceholder+" (from", 1)
	if !withheldAgree(t, "hermes", recorded, binary) {
		t.Error("a row of the entry's agent does not pass")
	}
	if withheldAgree(t, "cline", recorded, binary) {
		t.Error("a row of another agent passes")
	}
}

// withheldFormSamples returns, by entry label, a recorded text in which
// each entry of withheldForms shows.
func withheldFormSamples() map[string]string {
	return map[string]string{
		"tabby config.toml reader error":   "parser: config.toml is not valid TOML (Invalid value (at line 2, column 11))",
		"nanobot config.json reader error": "parser: config.json did not parse: Expecting value: line 1 column 1 (char 0)",
		"agent-zero chat.json reader error": "parser: chat.json did not parse (Expecting value: line 1 column 1 " +
			"(char 0)); log items recovered without history",
		"crush crush.json reader error":           "parser: crush.json is not a JSON object",
		"pi thinking row equal to a recorded key": `{"agent":"pi","text":"[redacted]"}`,
		"shellgpt settings line without an equals sign": `parser: unparseable line 1 (no "=" separator; not shown, ` +
			`the file holds the API key)`,
		"openclaw exec approvals reader error": "parser: exec_approvals_config raw_json unparseable (Expecting value: " +
			"line 1 column 22 (char 21); text withheld, it holds the approvals socket token)",
		"copilot-desktop account token": "account: ana host=github.com kind=github default=1 token_present=true",
		"hermes connections.json token": "remote backend: remote Homelab https://hermes.example.net " +
			"(from connections.json, file mtime)",
		"agent-zero telegram bot token": "session relayed: telegram bot opsbot chat 1 user 2 @ana " +
			"(from chat.json data, file mtime)",
		"agent-zero model override chat API key": "model: requested chat=openrouter/m utility=ollama/q " +
			"(chat override, as of file mtime)",
		"roo-code retry queue bearer token": "cloud upload queued: Task Message task=t type=telemetry retries=2 " +
			"url=https://app.example/api/events lastError=fetch failed (Roo Code Cloud retry queue, from f " +
			"roo.retryQueue, request timestamp)",
		"tabby integration access token": "source host: github.com integration acme kind=github api_base=- " +
			"(account not recorded; from integrations table, created_at)",
		"tabby config.toml API key": "model configured: chat m via http kind=openai/chat endpoint=https://h/v1 " +
			"(from config.toml [model.chat.http], file mtime)",
		"opencode share secret": "session synced: share SHARE05 url https://opncd.ai/share/SHARE05 " +
			"(from opencode-beta.db session_share, time_created)",
		"gemini-cli network request URL":             "network request: POST https://oauth2.googleapis.com/token (request t4r8z)",
		"detect and timeline .git/config remote URL": "  remotes: origin https://u:p@h/r.git\n",
	}
}

func TestEachWithheldFormIsFoundInItsRecordedTextAndFailsAsTheBinarys(t *testing.T) {
	texts := withheldFormSamples()
	for _, entry := range withheldForms() {
		t.Run(entry.label(), func(t *testing.T) {
			text, ok := texts[entry.label()]
			if !ok {
				t.Fatal("no sample text for this entry")
			}
			got := withheldEntriesIn(entry.scope, text, caseRun{inputs: func(*regexp.Regexp) bool { return true }})
			if entry.scope == parserScope {
				got = rowEntriesIn(withheldForms(), []Row{{Agent: Text(entry.agent), Text: Text(text)}})
			}
			if !slices.Equal(got, []string{entry.label()}) {
				t.Errorf("found %q", got)
			}
			agent := ""
			if entry.scope == parserScope {
				agent = entry.agent
			}
			run := caseRun{inputs: func(*regexp.Regexp) bool { return true }, requests: sampleRequests()}
			if withheldAgreeIn(t, run, agent, text, text) {
				t.Error("the recording's form passes as the binary's")
			}
			if entry.completed != nil {
				// The binary's side of a completed entry is compared as written
				// with the text completed gives; there is no token to write.
				return
			}
			compared, _ := entry.rewrite(recordedSide, text, run)
			if entry.scope == parserScope {
				compared, _ = entry.rowRewrite(recordedSide, entry.agent, text, run)
			}
			if withheldAgreeIn(t, run, agent, text, compared) {
				t.Errorf("the compared form %q written by the binary passes", compared)
			}
		})
	}
}

func TestOnlyDetectAndTimelineGetNoSecrets(t *testing.T) {
	tests := []struct {
		command string
		args    []string
		want    []string
	}{
		{"detect", []string{"--json", "in"},
			[]string{"--no-secrets", "--redaction-placeholder", replayPlaceholder, "--json", "in"}},
		{"timeline", []string{"--", "-in", "out"},
			[]string{"--no-secrets", "--redaction-placeholder", replayPlaceholder, "--", "-in", "out"}},
		{"fleet", []string{"a"}, []string{"a"}},
		{"catalog", nil, nil},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			if got := withSecretsWithheld(test.command, test.args); !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestTheBinarysSideOfACommandRunIsReadByWhereItsTextsComeFrom(t *testing.T) {
	retry := "cloud upload queued: Task Message task=t type=telemetry retries=2 url=https://app.example/api/events " +
		"lastError=fetch failed (Roo Code Cloud retry queue, from f roo.retryQueue, request timestamp)"
	line := func(text string) commandOutput {
		return commandOutput{outputs: map[string]string{"out/timeline.jsonl": `{"agent":"roo-code","text":"` + text +
			`"}` + "\n"}}
	}
	want := line(retry)
	compared := runForm(want, want)
	written := line(strings.Replace(retry, " lastError", "{added retry queue bearer token} lastError", 1))
	recordedRows := line(retry)
	recordedRows.source = recordedRowsRun
	tests := []struct {
		name  string
		got   commandOutput
		agree bool
	}{
		{"the binary's form built from the recording passes", compared, true},
		{"the binary writing the compared form's marker fails", written, false},
		{"the binary's own row passes", line(strings.Replace(retry, " lastError", " Authorization="+replayPlaceholder+" lastError", 1)),
			true},
		{"the binary's own row without the token fails", line(retry), false},
		{"a run over the recording's parser rows passes as recorded", recordedRows, true},
	}
	if !strings.Contains(compared.outputs["out/timeline.jsonl"], "{added retry queue bearer token}") {
		t.Fatalf("the compared form %q holds no marker", compared.outputs["out/timeline.jsonl"])
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if agree, _ := outputsAgree(t, want, test.got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestAThinkingRowEqualToAKeyIsWithheldOnlyWhereTheInputsHoldTheKey(t *testing.T) {
	request := []byte(`{"url":"https://oauth2.googleapis.com/token?key=redacted","method":"POST"}`)
	holding := func(pattern *regexp.Regexp) bool { return pattern.Match(request) }
	without := func(*regexp.Regexp) bool { return false }
	line := func(agent, text string) string {
		return `{"agent":"` + agent + `","text":"` + text + `"}` + "\n"
	}
	tests := []struct {
		name      string
		inputs    func(*regexp.Regexp) bool
		want, got string
		agree     bool
	}{
		{"the row withheld in a run over the request passes", holding, line("pi", "[redacted]"),
			line("pi", "["+replayPlaceholder+"]"), true},
		{"the row as written in a run over the request fails", holding, line("pi", "[redacted]"),
			line("pi", "[redacted]"), false},
		{"the row withheld in a run without the request fails", without, line("pi", "[redacted]"),
			line("pi", "["+replayPlaceholder+"]"), false},
		{"the row as written in a run without the request passes", without, line("pi", "[redacted]"),
			line("pi", "[redacted]"), true},
		{"the row withheld where the inputs are not known fails", nil, line("pi", "[redacted]"),
			line("pi", "["+replayPlaceholder+"]"), false},
		{"another agent's row withheld fails", holding, line("zed", "[redacted]"), line("zed", "["+replayPlaceholder+"]"), false},
		{"a row holding more than the key fails", holding, line("pi", "[redacted]"), line("pi", "["+replayPlaceholder+"] x"),
			false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := commandOutput{outputs: map[string]string{"out/timeline.jsonl": test.want}, inputs: test.inputs}
			got := commandOutput{outputs: map[string]string{"out/timeline.jsonl": test.got}}
			if agree, _ := outputsAgree(t, want, got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestNoEntryOrSourceOfThePackageWritesTheDefaultPlaceholder(t *testing.T) {
	if strings.Contains(replayPlaceholder, secrets.DefaultPlaceholder) {
		t.Fatalf("the replay's placeholder %q holds the default one", replayPlaceholder)
	}
	var texts []string
	for _, entry := range slices.Concat(withheldForms(), shownForms()) {
		texts = append(texts, entry.token)
		if entry.binary != nil {
			texts = append(texts, entry.binary.String())
		}
	}
	for _, entry := range withheldRows() {
		texts = append(texts, entry.text)
	}
	for _, text := range texts {
		if strings.Contains(text, secrets.DefaultPlaceholder) {
			t.Errorf("an entry holds the default placeholder: %q", text)
		}
	}
	files, err := osfile.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".go") {
			continue
		}
		data, err := osfile.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), secrets.DefaultPlaceholder) {
			t.Errorf("%s holds the default placeholder's text: write replayPlaceholder", file.Name())
		}
	}
}
