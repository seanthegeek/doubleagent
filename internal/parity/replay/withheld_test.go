//go:build parity

package replay

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// The recordings come from an analyzer that always withheld the secrets of
// the sites the binary shows by default, each site in its own form. So the
// replay runs every parser case and every detect and timeline command case
// with secrets withheld (withheldSecrets, withSecretsWithheld), and the
// binary's placeholder form is compared with the recording's form through
// difference 230.

// withheldSecrets returns the setting every replayed parser case runs
// under: secrets withheld as the default placeholder, as a run with
// --no-secrets gives.
func withheldSecrets() secrets.Setting {
	return secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder}
}

// withSecretsWithheld returns the arguments after a recorded detect or
// timeline command with --no-secrets ahead of them, so the run withholds
// secrets as every recorded run did; any other command's arguments are
// returned as they are.
func withSecretsWithheld(command string, args []string) []string {
	if command != "detect" && command != "timeline" {
		return args
	}
	return slices.Concat([]string{"--no-secrets"}, args)
}

// withheldDifference returns difference 230.
func withheldDifference() Difference {
	return Difference{Number: 230, Summary: "a withheld secret is written as the placeholder: at each site of " +
		"withheldForms, the recording's withheld form and the binary's form with REDACTED become one token " +
		"that keeps the rest of the row", Text: withheldText(recordedSide), Binary: withheldText(binarySide),
		Within: 102}
}

// withheldRowsDifference returns difference 231.
func withheldRowsDifference() Difference {
	return Difference{Number: 231, Summary: "a secret the recording never printed and had no row for is " +
		"shown in a row of its own: a parse may add one row of each entry of withheldRows, exactly as " +
		"written there, with REDACTED where the secret was", Added: withheldRowAdded}
}

// withheldRow is a row the binary adds for a secret the recording never
// printed and gave no row for: the agent whose parser gives it, a name,
// and the row's exact text with the default placeholder in place of the
// secret.
type withheldRow struct {
	agent, name, text string
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
		{"agent-zero", "connector settings", "connector settings: AGENT_ZERO_HOST=" + secrets.DefaultPlaceholder +
			" AGENT_ZERO_COMPUTER_USE_RESTORE_TOKEN=" + secrets.DefaultPlaceholder +
			" AGENT_ZERO_LAST_CONTEXT_HOST=" + secrets.DefaultPlaceholder +
			" AGENT_ZERO_LAST_CONTEXT_ID=" + secrets.DefaultPlaceholder + " (from .agent-zero/.env, file mtime)"},
		{"letta", "settings API key", "stored credentials: env.LETTA_API_KEY=" + secrets.DefaultPlaceholder +
			" (from .letta/settings.json, file mtime)"},
		{"shellgpt", "API key", "stored credentials: OPENAI_API_KEY=" + secrets.DefaultPlaceholder +
			" (from .config/shell_gpt/.sgptrc, file mtime)"},
		{"crush", "provider API key", "stored credentials: providers.anthropic.api_key=" + secrets.DefaultPlaceholder +
			" (from .crush/crush.json, file mtime)"},
		{"nanobot", "provider API key", "stored credentials: providers.openai.apiKey=" + secrets.DefaultPlaceholder +
			" (from .nanobot/config.json, file mtime)"},
		{"twinny", "inference provider API key", "model provider: p1 label=Ollama provider=ollama type=chat " +
			"model=qwen2.5-coder:7b apiKey=" + secrets.DefaultPlaceholder +
			" (from rjmacarthy.twinny twinny.inference-providers, file mtime)"},
		{"twinny", "active chat provider API key", "model provider: active chat p1 apiKey=" +
			secrets.DefaultPlaceholder + " (from rjmacarthy.twinny twinny.active-chat-provider, file mtime)"},
		{"tabby", "user auth token", "tabby user: alice@example.com (Alice) id=1 auth_token=" +
			secrets.DefaultPlaceholder + " (from users table, created_at)"},
		{"tabby", "server record token", "tabby server: http://localhost:8080 token=" + secrets.DefaultPlaceholder +
			" (from state.vscdb TabbyML.vscode-tabby server.serverRecords, no times)"},
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
		index := slices.IndexFunc(got, func(row Row) bool {
			return row.TurnType == "system" && string(row.SourceFile) == parse.file &&
				string(row.Text) == entry.text && !slices.Contains(want, row)
		})
		if index >= 0 {
			got = slices.Delete(slices.Clone(got), index, index+1)
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
// the token both become.
type withheldForm struct {
	scope, agent, name string
	lines              *regexp.Regexp
	recorded, binary   *regexp.Regexp
	token              string
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

// rewrite returns text with the entry's form of one side as its token,
// only on the entry's lines when it names them, and the count.
func (f withheldForm) rewrite(from side, text string) (string, int) {
	pattern := f.binary
	if from == recordedSide {
		pattern = f.recorded
	}
	found := 0
	replace := func(part string) string {
		found += len(pattern.FindAllStringIndex(part, -1))
		return pattern.ReplaceAllString(part, f.token)
	}
	if f.lines == nil {
		return replace(text), found
	}
	return f.lines.ReplaceAllStringFunc(text, replace), found
}

// withheldForms returns the entries of difference 230. A parser that comes
// to withhold a secret the recording gave in another form adds an entry,
// and a run of every case fails for an entry no case of its scope used.
// The binary's form always holds REDACTED where the secret was, so a
// binary that shows the secret, or prints the recording's form, still
// fails.
func withheldForms() []withheldForm {
	form := func(agent, name, recorded, binary, token string) withheldForm {
		return withheldForm{scope: parserScope, agent: agent, name: name, recorded: regexp.MustCompile(recorded),
			binary: regexp.MustCompile(binary), token: token}
	}
	pythonJSON := `(?:` + strings.Join(pythonJSONMessages(), "|") + `): line ([0-9]+) column ([0-9]+) \(char [0-9]+\)`
	goJSON := `invalid JSON at line ([0-9]+), column ([0-9]+): ` + secrets.DefaultPlaceholder
	position := "{withheld at line ${2}, column ${3}}"
	schemeText := `[a-z][a-z0-9+.-]*://`
	scheme := "(" + schemeText + ")"
	return []withheldForm{
		form("tabby", "config.toml reader error",
			`(parser: config\.toml is not valid TOML \()(?:`+strings.Join(tomlMessages(), "|")+`)[^"\n]*? `+
				`\(at line ([0-9]+), column ([0-9]+)\)\)`,
			`(parser: config\.toml is not valid TOML \()invalid TOML at line ([0-9]+), column ([0-9]+): `+
				secrets.DefaultPlaceholder+`\)`,
			"${1}"+position+")"),
		form("nanobot", "config.json reader error",
			`(parser: config\.json did not parse: )`+pythonJSON+`("|\n|\z)`,
			`(parser: config\.json did not parse: )`+goJSON+`("|\n|\z)`,
			"${1}"+position+"${4}"),
		// The recording names a crush.json that is not JSON without the
		// reader's error; the binary adds it, withheld.
		form("crush", "crush.json reader error",
			`(parser: [^"\n]*crush\.json is not a JSON object)("|\n|\z)`,
			`(parser: [^"\n]*crush\.json is not a JSON object)(?: \(invalid JSON at line [0-9]+, column [0-9]+: `+
				secrets.DefaultPlaceholder+`\))?("|\n|\z)`,
			"${1}${2}"),
		form("openclaw", "exec approvals reader error",
			`(exec_approvals_config raw_json unparseable \()`+pythonJSON+`; text withheld, it holds the approvals `+
				`socket token\)`,
			`(exec_approvals_config raw_json unparseable \()`+goJSON+`\)`,
			"${1}"+position+")"),
		// The recording said only that an account's token was present; the
		// binary gives the token, withheld.
		form("copilot-desktop", "account token",
			`(account: [^"\n]* )token_present=true("|\n|\z)`,
			`(account: [^"\n]* )token=`+secrets.DefaultPlaceholder+`("|\n|\z)`,
			"${1}{withheld token}${2}"),
		// The recording gave a desktop connection without the token stored
		// in plain text beside it; the binary adds it, withheld.
		form("hermes", "connections.json token",
			`(remote backend: [^"\n]*?)( \(from connections\.json, file mtime\))`,
			`(remote backend: [^"\n]*?)(?: token=`+secrets.DefaultPlaceholder+`)?( \(from connections\.json, `+
				`file mtime\))`,
			"${1}${2}"),
		// The recording gave a Telegram chat's relay without the bot's token
		// and webhook secret; the binary adds them, withheld.
		form("agent-zero", "telegram bot token",
			`(session relayed: telegram bot [^"\n]*?)( \(from chat\.json data, file mtime\))`,
			`(session relayed: telegram bot [^"\n]*?)(?: token=`+secrets.DefaultPlaceholder+`)?`+
				`(?: webhook_secret=`+secrets.DefaultPlaceholder+`)?( \(from chat\.json data, file mtime\))`,
			"${1}${2}"),
		// The recording gave a chat override's slots without the API key a
		// slot holds; the binary adds it after the slot, withheld.
		form("agent-zero", "model override API key",
			`(model: requested [^"\n]*?)( \(chat override, as of file mtime\))`,
			`(model: requested [^"\n]*?)(?: (?:chat|utility|embedding)\.api_key=`+secrets.DefaultPlaceholder+
				`)?((?: [^"\n]*?)? \(chat override, as of file mtime\))`,
			"${1}${2}"),
		// The recording gave a retried upload without its bearer token; the
		// binary adds it after the URL, withheld.
		form("roo-code", "retry queue bearer token",
			`(cloud upload queued: [^"\n]*? url=[^ "\n]*)( [^"\n]*?\(Roo Code Cloud retry queue, )`,
			`(cloud upload queued: [^"\n]*? url=[^ "\n]*)(?: Authorization=`+secrets.DefaultPlaceholder+`)?`+
				`( [^"\n]*?\(Roo Code Cloud retry queue, )`,
			"${1}${2}"),
		// The recording gave the exec approvals settings without the
		// approvals socket's token; the binary adds it to each, withheld.
		form("openclaw", "exec approvals socket token",
			`(permission setting: [^"\n]*?)( \(from state/openclaw\.sqlite exec_approvals_config, updated_at_ms\))`,
			`(permission setting: [^"\n]*?)(?: socket\.token=`+secrets.DefaultPlaceholder+`)?`+
				`( \(from state/openclaw\.sqlite exec_approvals_config, updated_at_ms\))`,
			"${1}${2}"),
		// The recording gave an integration without its access token; the
		// binary adds it, withheld.
		form("tabby", "integration access token",
			`(source host: [^"\n]* api_base=[^ "\n]*)( \(account not recorded; from integrations table, created_at\))`,
			`(source host: [^"\n]* api_base=[^ "\n]*)(?: access_token=`+secrets.DefaultPlaceholder+`)?`+
				`( \(account not recorded; from integrations table, created_at\))`,
			"${1}${2}"),
		// The recording gave a configured model without its API key; the
		// binary adds it, withheld.
		form("tabby", "config.toml API key",
			`(model configured: [^"\n]*?)( [^"\n]*\(from config\.toml \[model\.)`,
			`(model configured: [^"\n]*?) api_key=`+secrets.DefaultPlaceholder+`( [^"\n]*\(from config\.toml \[model\.)`,
			"${1}${2}"),
		// The recording gave a session's share without its secret; the
		// binary adds it, withheld.
		form("opencode", "share secret",
			`(session synced: share [^"\n]* url [^ "\n]*)( \(from [^"\n]* session_share, time_created\))`,
			`(session synced: share [^"\n]* url [^ "\n]*)(?: secret=`+secrets.DefaultPlaceholder+`)?`+
				`( \(from [^"\n]* session_share, time_created\))`,
			"${1}${2}"),
		// The recording cut a network request's URL at its query, which can
		// carry an API key; the binary gives the whole URL, the key
		// withheld. Only the queries of the recorded logs are taken, so a
		// key shown, or another parameter, still fails.
		form("gemini-cli", "network request URL",
			`(network (?:request|response|error): [A-Z]+ https?://[^ ?#"\n]*)`,
			`(network (?:request|response|error): [A-Z]+ https?://[^ ?#"\n]*)`+
				`(?:\?(?:alt=sse|key=`+secrets.DefaultPlaceholder+`)(?:&(?:alt=sse|key=`+secrets.DefaultPlaceholder+`))*)?`,
			"${1}"),
		// The recording printed a repository's remote URLs as written; the
		// binary run with --no-secrets withholds their user information, on
		// detect's remotes: line and in the urls and pushurls arrays of
		// detect --json and inventory.json, one URL per line.
		{scope: commandScope, agent: "detect and timeline", name: ".git/config remote URL",
			lines:    regexp.MustCompile(`(?m)^(?:  remotes: [^\n]*|[ \t]*"` + schemeText + `[^"\n]*",?)$`),
			recorded: regexp.MustCompile(scheme + `[^\s"@/]+@`),
			binary:   regexp.MustCompile(scheme + secrets.DefaultPlaceholder + `@`), token: "${1}{withheld}@"},
	}
}

// withheldText returns the rewrite of difference 230 for one side: each
// entry's form of that side as its token, and the count.
func withheldText(from side) func(text string) (string, int) {
	return func(text string) (string, int) {
		total := 0
		for _, entry := range withheldForms() {
			var found int
			text, found = entry.rewrite(from, text)
			total += found
		}
		return text, total
	}
}

// withheldEntriesIn returns the labels of the entries of scope whose
// recorded form shows in text.
func withheldEntriesIn(scope, text string) []string {
	var labels []string
	for _, entry := range withheldForms() {
		if _, found := entry.rewrite(recordedSide, text); found > 0 && entry.scope == scope {
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
	parse := parsedFile{"agent-zero", file, rel}
	origin := Row{TurnType: "system", SourceFile: file, SessionID: "RemCtx01",
		Text: "session origin: remote agent-zero instance https://a0.example.net context RemCtx01 " +
			"(from .agent-zero/.env, file mtime)"}
	settings := Row{TurnType: "system", SourceFile: file, Text: Text(withheldRows()[0].text)}
	shown := settings
	shown.Text = Text(strings.Replace(string(settings.Text), "REDACTED", "fixture-restore-token", 1))
	elsewhere := settings
	elsewhere.SourceFile = "/h/other.env"
	notSystem := settings
	notSystem.TurnType = "user"
	changed := origin
	changed.Text = Text(strings.Replace(string(origin.Text), "RemCtx01", "RemCtx02", 1))
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
		{"the row added by another agent's parse fails", parsedFile{"hermes", file, rel}, []Row{origin},
			[]Row{settings, origin}, false},
		{"a changed row beside it fails", parse, []Row{origin}, []Row{settings, changed}, false},
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
	parse := parsedFile{"twinny", file, rel}
	var rows []Row
	for _, entry := range withheldRows() {
		if entry.agent == "twinny" {
			rows = append(rows, Row{TurnType: "system", SourceFile: file, Text: Text(entry.text)})
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

func TestAWithheldSecretIsComparedAsItsPlaceholderAndNothingElse(t *testing.T) {
	tomlRecorded := "parser: config.toml is not valid TOML (Invalid value (at line 2, column 11))"
	tomlBinary := "parser: config.toml is not valid TOML (invalid TOML at line 2, column 11: REDACTED)"
	jsonRecorded := "parser: config.json did not parse: Expecting value: line 1 column 37 (char 36)"
	jsonBinary := "parser: config.json did not parse: invalid JSON at line 1, column 37: REDACTED"
	crushRecorded := "parser: .config/crush/crush.json is not a JSON object"
	crushBinary := crushRecorded + " (invalid JSON at line 1, column 38: REDACTED)"
	execRecorded := "parser: exec_approvals_config raw_json unparseable (Expecting value: line 1 column 22 " +
		"(char 21); text withheld, it holds the approvals socket token)"
	execBinary := "parser: exec_approvals_config raw_json unparseable (invalid JSON at line 1, column 22: REDACTED)"
	remoteRecorded := "  remotes: origin git@h:o/r.git (push https://alice:TOKEN@github.com/o/r.git)\n"
	remoteBinary := strings.Replace(remoteRecorded, "alice:TOKEN", "REDACTED", 1)
	jsonRemote := "          \"urls\": [\n            \"https://alice:TOKEN@github.com/o/r.git\"\n"
	accountRecorded := "account: ana host=github.com kind=github default=1 token_present=true"
	accountBinary := "account: ana host=github.com kind=github default=1 token=REDACTED"
	accountAbsent := "account: ana host=github.com kind=github default=1 token_present=false"
	connectionRecorded := "remote backend: remote Homelab https://hermes.example.net (from connections.json, " +
		"file mtime)"
	connectionBinary := strings.Replace(connectionRecorded, " (from", " token=REDACTED (from", 1)
	telegramRecorded := "session relayed: telegram bot opsbot chat 1 user 2 @ana (from chat.json data, file mtime)"
	telegramBinary := strings.Replace(telegramRecorded, " (from", " token=REDACTED (from", 1)
	overrideRecorded := "model: requested chat=openrouter/m utility=ollama/q (chat override, as of file mtime)"
	overrideBinary := strings.Replace(overrideRecorded, " utility", " chat.api_key=REDACTED utility", 1)
	retryRecorded := "cloud upload queued: Task Message task=t type=telemetry retries=2 " +
		"url=https://app.example/api/events lastError=fetch failed (Roo Code Cloud retry queue, from f " +
		"roo.retryQueue, request timestamp)"
	retryBinary := strings.Replace(retryRecorded, " lastError", " Authorization=REDACTED lastError", 1)
	socketRecorded := "permission setting: defaults.security=allowlist (from state/openclaw.sqlite " +
		"exec_approvals_config, updated_at_ms)"
	socketBinary := strings.Replace(socketRecorded, " (from", " socket.token=REDACTED (from", 1)
	integrationRecorded := "source host: github.com integration acme kind=github api_base=- " +
		"(account not recorded; from integrations table, created_at)"
	integrationBinary := strings.Replace(integrationRecorded, " (account", " access_token=REDACTED (account", 1)
	modelRecorded := "model configured: chat m via http kind=openai/chat endpoint=https://h/v1 " +
		"supported_models=m (from config.toml [model.chat.http], file mtime)"
	modelBinary := strings.Replace(modelRecorded, " supported", " api_key=REDACTED supported", 1)
	shareRecorded := "session synced: share SHARE05 url https://opncd.ai/share/SHARE05 " +
		"(from opencode-beta.db session_share, time_created)"
	shareBinary := strings.Replace(shareRecorded, " (from", " secret=REDACTED (from", 1)
	networkRecorded := "network request: POST https://oauth2.googleapis.com/token (request t4r8z)"
	networkBinary := strings.Replace(networkRecorded, "/token", "/token?key=REDACTED", 1)
	tests := []struct {
		name, want, got string
		agree           bool
	}{
		{"a remote in its placeholder form passes", remoteRecorded, remoteBinary, true},
		{"a remote showing its credentials fails", remoteRecorded, remoteRecorded, false},
		{"a remote of another host fails", remoteRecorded, strings.Replace(remoteBinary, "github.com", "gitlab.com", 1),
			false},
		{"a remote of another name fails", remoteRecorded, strings.Replace(remoteBinary, "origin", "upstream", 1), false},
		{"a remote URL in JSON in its placeholder form passes", jsonRemote,
			strings.Replace(jsonRemote, "alice:TOKEN", "REDACTED", 1), true},
		{"a URL with credentials in a row's text is not this entry's", "{\"text\":\"https://alice:TOKEN@h/\"}\n",
			"{\"text\":\"https://REDACTED@h/\"}\n", false},
		{"a TOML error in its placeholder form passes", tomlRecorded, tomlBinary, true},
		{"a TOML error showing the reader's detail fails", tomlRecorded,
			strings.Replace(tomlBinary, "REDACTED", `expected value but found "sk" instead`, 1), false},
		{"a TOML error at another column fails", tomlRecorded, strings.Replace(tomlBinary, "11", "12", 1), false},
		{"a TOML error of another file fails", tomlRecorded, strings.Replace(tomlBinary, "config", "other", 1), false},
		{"a JSON error in its placeholder form passes", jsonRecorded, jsonBinary, true},
		{"a JSON error showing the reader's detail fails", jsonRecorded,
			strings.Replace(jsonBinary, "REDACTED", "invalid character 's' looking for beginning of value", 1), false},
		{"a JSON error on another line fails", jsonRecorded, strings.Replace(jsonBinary, "line 1", "line 2", 1), false},
		{"a JSON error in another placeholder fails", jsonRecorded, strings.Replace(jsonBinary, "REDACTED", "x", 1),
			false},
		{"the recording's wording printed by the binary fails", jsonRecorded, jsonRecorded, false},
		{"a crush.json error in its placeholder form passes", crushRecorded, crushBinary, true},
		{"a crush.json that is valid JSON passes as written", crushRecorded, crushRecorded, true},
		{"a crush.json error showing the reader's detail fails", crushRecorded,
			strings.Replace(crushBinary, "REDACTED", "invalid character 's'", 1), false},
		{"a crush.json error naming another file fails", crushRecorded,
			strings.Replace(crushBinary, ".config", ".local", 1), false},
		{"an exec approvals error in its placeholder form passes", execRecorded, execBinary, true},
		{"an exec approvals error showing the reader's detail fails", execRecorded,
			strings.Replace(execBinary, "REDACTED", "invalid character 's'", 1), false},
		{"an exec approvals error at another column fails", execRecorded,
			strings.Replace(execBinary, "22", "23", 1), false},
		{"an exec approvals error of another table fails", execRecorded,
			strings.Replace(execBinary, "exec_approvals_config", "exec_approvals", 1), false},
		{"an account token in its placeholder form passes", accountRecorded, accountBinary, true},
		{"an account token shown fails", accountRecorded,
			strings.Replace(accountBinary, "REDACTED", "FIXTURE-TOKEN", 1), false},
		{"an account token in the recording's form fails", accountRecorded, accountRecorded, false},
		{"an account token of another login fails", accountRecorded, strings.Replace(accountBinary, "ana", "bo", 1),
			false},
		{"an absent account token passes as written", accountAbsent, accountAbsent, true},
		{"a bot token in its placeholder form passes", telegramRecorded, telegramBinary, true},
		{"a bot token shown fails", telegramRecorded, strings.Replace(telegramBinary, "REDACTED", "123:AA", 1), false},
		{"a bot of another chat fails", telegramRecorded, strings.Replace(telegramBinary, "chat 1", "chat 9", 1),
			false},
		{"a slot's API key in its placeholder form passes", overrideRecorded, overrideBinary, true},
		{"a slot's API key shown fails", overrideRecorded, strings.Replace(overrideBinary, "REDACTED", "sk-1", 1),
			false},
		{"a slot of another model fails", overrideRecorded, strings.Replace(overrideBinary, "ollama/q", "ollama/r", 1),
			false},
		{"a retried upload's token in its placeholder form passes", retryRecorded, retryBinary, true},
		{"a retried upload's token shown fails", retryRecorded,
			strings.Replace(retryBinary, "REDACTED", "Bearer eyJ", 1), false},
		{"a retried upload of another URL fails", retryRecorded,
			strings.Replace(retryBinary, "api/events", "api/other", 1), false},
		{"a retried upload of another error fails", retryRecorded,
			strings.Replace(retryBinary, "fetch failed", "timeout", 1), false},
		{"a socket token in its placeholder form passes", socketRecorded, socketBinary, true},
		{"a socket token shown fails", socketRecorded, strings.Replace(socketBinary, "REDACTED", "tok", 1), false},
		{"a socket token beside another setting fails", socketRecorded,
			strings.Replace(socketBinary, "allowlist", "full", 1), false},
		{"a request's withheld key passes", networkRecorded, networkBinary, true},
		{"a request's query of the recorded logs passes", networkRecorded,
			strings.Replace(networkRecorded, "/token", "/token?alt=sse", 1), true},
		{"a request's key shown fails", networkRecorded, strings.Replace(networkBinary, "REDACTED", "AIza1", 1), false},
		{"a request's other parameter fails", networkRecorded,
			strings.Replace(networkRecorded, "/token", "/token?x=1", 1), false},
		{"a request to another path fails", networkRecorded, strings.Replace(networkBinary, "/token", "/tokens", 1),
			false},
		{"a share's secret in its placeholder form passes", shareRecorded, shareBinary, true},
		{"a share's secret shown fails", shareRecorded, strings.Replace(shareBinary, "REDACTED", "sec", 1), false},
		{"a share of another id fails", shareRecorded, strings.Replace(shareBinary, "SHARE05 url", "SHARE06 url", 1),
			false},
		{"a model's API key in its placeholder form passes", modelRecorded, modelBinary, true},
		{"a model's API key shown fails", modelRecorded, strings.Replace(modelBinary, "REDACTED", "sk-1", 1), false},
		{"a model of another endpoint fails", modelRecorded, strings.Replace(modelBinary, "h/v1", "g/v1", 1), false},
		{"an integration's token in its placeholder form passes", integrationRecorded, integrationBinary, true},
		{"an integration's token shown fails", integrationRecorded,
			strings.Replace(integrationBinary, "REDACTED", "ghp_x", 1), false},
		{"an integration of another kind fails", integrationRecorded,
			strings.Replace(integrationBinary, "kind=github", "kind=gitlab", 1), false},
		{"a connection's token in its placeholder form passes", connectionRecorded, connectionBinary, true},
		{"a connection's token shown fails", connectionRecorded,
			strings.Replace(connectionBinary, "REDACTED", "not-real", 1), false},
		{"a connection of another URL fails", connectionRecorded,
			strings.Replace(connectionBinary, "hermes.example.net", "other.example", 1), false},
		{"a connection's token in another placeholder fails", connectionRecorded,
			strings.Replace(connectionBinary, "REDACTED", "x", 1), false},
		{"an absent account token said to be present fails", accountAbsent, accountBinary, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &recordingTB{TB: t}
			want, got := NormaliseRows(reporter, "n", []int{102, 230}, []Row{{Text: Text(test.want)}},
				[]Row{{Text: Text(test.got)}})
			if agree := want[0].Text == got[0].Text; agree != test.agree {
				t.Errorf("agree %v, want %v: %q and %q", agree, test.agree, want[0].Text, got[0].Text)
			}
		})
	}
}

func TestEachWithheldFormIsFoundInItsRecordedText(t *testing.T) {
	texts := map[string]string{
		"tabby config.toml reader error":   "parser: config.toml is not valid TOML (Invalid value (at line 2, column 11))",
		"nanobot config.json reader error": "parser: config.json did not parse: Expecting value: line 1 column 1 (char 0)",
		"crush crush.json reader error":    "parser: crush.json is not a JSON object",
		"openclaw exec approvals reader error": "parser: exec_approvals_config raw_json unparseable (Expecting value: " +
			"line 1 column 22 (char 21); text withheld, it holds the approvals socket token)",
		"copilot-desktop account token": "account: ana host=github.com kind=github default=1 token_present=true",
		"hermes connections.json token": "remote backend: remote Homelab https://hermes.example.net " +
			"(from connections.json, file mtime)",
		"agent-zero telegram bot token": "session relayed: telegram bot opsbot chat 1 user 2 @ana " +
			"(from chat.json data, file mtime)",
		"agent-zero model override API key": "model: requested chat=openrouter/m utility=ollama/q " +
			"(chat override, as of file mtime)",
		"roo-code retry queue bearer token": "cloud upload queued: Task Message task=t type=telemetry retries=2 " +
			"url=https://app.example/api/events lastError=fetch failed (Roo Code Cloud retry queue, from f " +
			"roo.retryQueue, request timestamp)",
		"openclaw exec approvals socket token": "permission setting: defaults.security=allowlist " +
			"(from state/openclaw.sqlite exec_approvals_config, updated_at_ms)",
		"tabby integration access token": "source host: github.com integration acme kind=github api_base=- " +
			"(account not recorded; from integrations table, created_at)",
		"tabby config.toml API key": "model configured: chat m via http kind=openai/chat endpoint=https://h/v1 " +
			"(from config.toml [model.chat.http], file mtime)",
		"opencode share secret": "session synced: share SHARE05 url https://opncd.ai/share/SHARE05 " +
			"(from opencode-beta.db session_share, time_created)",
		"gemini-cli network request URL":             "network request: POST https://oauth2.googleapis.com/token (request t4r8z)",
		"detect and timeline .git/config remote URL": "  remotes: origin https://u:p@h/r.git\n",
	}
	for _, entry := range withheldForms() {
		t.Run(entry.label(), func(t *testing.T) {
			text, ok := texts[entry.label()]
			if !ok {
				t.Fatal("no sample text for this entry")
			}
			if got := withheldEntriesIn(entry.scope, text); !slices.Equal(got, []string{entry.label()}) {
				t.Errorf("found %q", got)
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
		{"detect", []string{"--json", "in"}, []string{"--no-secrets", "--json", "in"}},
		{"timeline", []string{"--", "-in", "out"}, []string{"--no-secrets", "--", "-in", "out"}},
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
