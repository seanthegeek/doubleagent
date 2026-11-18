package timeline

// The rows and sessions are the documented line forms of timeline.jsonl and
// sessions.jsonl.

// rowLineCase is a row and its timeline.jsonl line.
type rowLineCase struct {
	row  Row
	want string
}

// rowLineCases returns rows and their timeline.jsonl lines.
func rowLineCases() []rowLineCase {
	return []rowLineCase{
		{Row{}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"\",\"source_file\":\"\",\"source_line\":0,\"recovered\":\"\"}\n"},
		{Row{TimestampUTC: "2026-10-01T08:00:00.000Z", Host: "h", User: "alice", Agent: "claude-code", SessionID: "s1", ProjectPath: "/srv/p", GitBranch: "main", TurnType: "user", Model: "m", ToolName: "Bash", ToolUseID: "t1", Text: "run the tests", SourceFile: "/a.jsonl", SourceLine: 7, Recovered: "yes"}, "{\"timestamp_utc\":\"2026-10-01T08:00:00.000Z\",\"host\":\"h\",\"user\":\"alice\",\"agent\":\"claude-code\",\"session_id\":\"s1\",\"project_path\":\"/srv/p\",\"git_branch\":\"main\",\"turn_type\":\"user\",\"model\":\"m\",\"tool_name\":\"Bash\",\"tool_use_id\":\"t1\",\"text\":\"run the tests\",\"source_file\":\"/a.jsonl\",\"source_line\":7,\"recovered\":\"yes\"}\n"},
		{Row{Text: "quote \" backslash \\ slash / newline \n cr \r tab \t bs \x08 ff \x0c"}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"quote \\\" backslash \\\\ slash / newline \\n cr \\r tab \\t bs \\b ff \\f\",\"source_file\":\"\",\"source_line\":0,\"recovered\":\"\"}\n"},
		{Row{Text: "controls \x00\x01\x1f del \x7f"}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"controls \\u0000\\u0001\\u001f del \x7f\",\"source_file\":\"\",\"source_line\":0,\"recovered\":\"\"}\n"},
		{Row{Text: "non-ASCII caf\U000000E9 \U00004E2D\U00006587 emoji \U0001F600 nbsp \U000000A0"}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"non-ASCII caf\U000000E9 \U00004E2D\U00006587 emoji \U0001F600 nbsp \U000000A0\",\"source_file\":\"\",\"source_line\":0,\"recovered\":\"\"}\n"},
		{Row{Text: "separators \U00002028 and \U00002029 and nel \U00000085"}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"separators \\u2028 and \\u2029 and nel \U00000085\",\"source_file\":\"\",\"source_line\":0,\"recovered\":\"\"}\n"},
		{Row{Text: "invalid UTF-8 \xed\xa0\xbd caf\U000000E9 \U00002028 \x7f"}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"invalid UTF-8 \U0000FFFD\U0000FFFD\U0000FFFD caf\U000000E9 \\u2028 \x7f\",\"source_file\":\"\",\"source_line\":0,\"recovered\":\"\"}\n"},
		{Row{Text: "<script>&amp;</script>"}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"<script>&amp;</script>\",\"source_file\":\"\",\"source_line\":0,\"recovered\":\"\"}\n"},
		{Row{SourceLine: -3}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"\",\"source_file\":\"\",\"source_line\":-3,\"recovered\":\"\"}\n"},
		{Row{SourceLine: 1099511627776}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"\",\"source_file\":\"\",\"source_line\":1099511627776,\"recovered\":\"\"}\n"},
		{Row{Text: "bom \U0000FEFF and replacement \U0000FFFD"}, "{\"timestamp_utc\":\"\",\"host\":\"\",\"user\":\"\",\"agent\":\"\",\"session_id\":\"\",\"project_path\":\"\",\"git_branch\":\"\",\"turn_type\":\"\",\"model\":\"\",\"tool_name\":\"\",\"tool_use_id\":\"\",\"text\":\"bom \U0000FEFF and replacement \U0000FFFD\",\"source_file\":\"\",\"source_line\":0,\"recovered\":\"\"}\n"},
	}
}

// sessionCase is a set of rows, the mark agents and the sessions.jsonl
// lines of their summaries.
type sessionCase struct {
	name  string
	rows  []Row
	marks MarkAgents
	want  string
}

// sessionCases returns the session scenarios.
func sessionCases() []sessionCase {
	return []sessionCase{
		{"counts, models, first and last times", []Row{
			{TimestampUTC: "2026-10-01T08:00:02.000Z", Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "user", Model: "b", SourceFile: "/t"},
			{TimestampUTC: "2026-10-01T08:00:01.000Z", Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "assistant", Model: "a", SourceFile: "/t"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "assistant", Model: "b", SourceFile: "/h"},
			{TimestampUTC: "2026-10-01T08:00:03.000Z", Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "tool_use"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "tool_result"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "thinking"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "system", Text: "note"},
		}, MarkAgents{}, "{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"s\",\"project_path\":\"\",\"first_timestamp_utc\":\"2026-10-01T08:00:01.000Z\",\"last_timestamp_utc\":\"2026-10-01T08:00:03.000Z\",\"models\":[\"b\",\"a\"],\"user_turns\":1,\"assistant_turns\":2,\"tool_calls\":1,\"source_file\":\"/t\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n"},
		{"the source with most rows wins, the first on a tie", []Row{
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "user", SourceFile: "/history"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "user", SourceFile: "/transcript"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "user", SourceFile: "/transcript"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "user", SourceFile: "/history"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "user"},
		}, MarkAgents{}, "{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"s\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":5,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"/history\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n"},
		{"a scratch row's path is used only without another", []Row{
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "system", Text: "scratch directory: tmp"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", ProjectPath: "/scratch", TurnType: "system", Text: "scratch directory: x"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: "user"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "t", ProjectPath: "/scratch", TurnType: "system", Text: "scratch directory: x"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "t", ProjectPath: "/project", TurnType: "user"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "t", ProjectPath: "/other", TurnType: "user"},
		}, MarkAgents{}, "{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"s\",\"project_path\":\"/scratch\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"t\",\"project_path\":\"/project\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":2,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n"},
		{"marks and their defaults", []Row{
			{Host: "h", User: "u", Agent: "copilot-cli", SessionID: "c", TurnType: "user"},
			{Host: "h", User: "u", Agent: "copilot-cli", SessionID: "d", TurnType: "system", Text: "session synced: account"},
			{Host: "h", User: "u", Agent: "copilot-cli", SessionID: "d", TurnType: "system", Text: "session relayed: telegram"},
			{Host: "h", User: "u", Agent: "copilot-cli", SessionID: "d", TurnType: "system", Text: "session origin: remote cloud"},
			{Host: "h", User: "u", Agent: "copilot-cli", SessionID: "e", TurnType: "user", Text: "session synced: not a system row"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "f", TurnType: "user"},
		}, MarkAgents{Synced: map[string]bool{"copilot-cli": true}, Relayed: map[string]bool{"copilot-cli": true}, Origin: map[string]bool{"copilot-cli": true}}, "{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"f\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"copilot-cli\",\"session_id\":\"c\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"no\",\"relayed\":\"no\",\"origin\":\"local\",\"recovered\":\"\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"copilot-cli\",\"session_id\":\"d\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":0,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"yes\",\"relayed\":\"yes\",\"origin\":\"remote\",\"recovered\":\"\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"copilot-cli\",\"session_id\":\"e\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"no\",\"relayed\":\"no\",\"origin\":\"local\",\"recovered\":\"\"}\n"},
		{"recovered counts event rows only", []Row{
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "all", TurnType: "user", Recovered: "yes"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "all", TurnType: "system", Text: "parser: x"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "some", TurnType: "user", Recovered: "yes"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "some", TurnType: "assistant"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "none", TurnType: "user"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "parser", TurnType: "system", Text: "parser: bad", Recovered: "yes"},
			{Host: "h", User: "u", Agent: "other", SessionID: "other", TurnType: "user"},
		}, MarkAgents{Secondary: map[string]bool{"claude-code": true}}, "{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"all\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"yes\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"none\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"no\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"parser\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":0,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"no\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"some\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":1,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"partial\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"other\",\"session_id\":\"other\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n"},
		{"sessions are keyed by host, user, agent and id, sorted with ties in order", []Row{
			{Host: "h2", User: "u", Agent: "claude-code", SessionID: "b", TurnType: "user"},
			{Host: "h1", User: "u", Agent: "claude-code", SessionID: "b", TurnType: "user"},
			{TimestampUTC: "2026-10-01T08:00:00.000Z", Host: "h", User: "u", Agent: "claude-code", SessionID: "a", TurnType: "user"},
			{TimestampUTC: "2026-10-01T08:00:00.000Z", Host: "h", User: "u", Agent: "aider", SessionID: "z", TurnType: "user"},
			{Host: "h", User: "v", Agent: "claude-code", SessionID: "b", TurnType: "user"},
			{Host: "h", User: "u", Agent: "claude-code", TurnType: "user"},
			{Host: "h", User: "u", Agent: "claude-code", SessionID: "c", TurnType: "user"},
		}, MarkAgents{}, "{\"host\":\"h2\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"b\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n{\"host\":\"h1\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"b\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n{\"host\":\"h\",\"user\":\"v\",\"agent\":\"claude-code\",\"session_id\":\"b\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"c\",\"project_path\":\"\",\"first_timestamp_utc\":\"\",\"last_timestamp_utc\":\"\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"aider\",\"session_id\":\"z\",\"project_path\":\"\",\"first_timestamp_utc\":\"2026-10-01T08:00:00.000Z\",\"last_timestamp_utc\":\"2026-10-01T08:00:00.000Z\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n{\"host\":\"h\",\"user\":\"u\",\"agent\":\"claude-code\",\"session_id\":\"a\",\"project_path\":\"\",\"first_timestamp_utc\":\"2026-10-01T08:00:00.000Z\",\"last_timestamp_utc\":\"2026-10-01T08:00:00.000Z\",\"models\":[],\"user_turns\":1,\"assistant_turns\":0,\"tool_calls\":0,\"source_file\":\"\",\"synced\":\"\",\"relayed\":\"\",\"origin\":\"\",\"recovered\":\"\"}\n"},
	}
}

// pathCase is a project path, its canonical form, the key this package
// gives, whether it is a Windows path, and the key a full Unicode lowering
// would give where it differs: this package lowers ASCII letters only.
type pathCase struct {
	input, canonical, key string
	windows               bool
	unicodeKey            string
}

// pathCases returns the project path cases.
func pathCases() []pathCase {
	return []pathCase{
		{"c:/Users/sean/demo", "C:\\Users\\sean\\demo", "c:\\users\\sean\\demo", true, ""},
		{"C:\\users\\sean\\demo2", "C:\\users\\sean\\demo2", "c:\\users\\sean\\demo2", true, ""},
		{"C:/users//sean\\\\demo2", "C:\\users\\sean\\demo2", "c:\\users\\sean\\demo2", true, ""},
		{"C:\\Users\\sean\\demo\\", "C:\\Users\\sean\\demo", "c:\\users\\sean\\demo", true, ""},
		{"c:/Users/sean/demo/", "C:\\Users\\sean\\demo", "c:\\users\\sean\\demo", true, ""},
		{"C:\\", "C:\\", "c:\\", true, ""},
		{"C:/", "C:\\", "c:\\", true, ""},
		{"c:\\\\", "C:\\", "c:\\", true, ""},
		{"\\\\?\\C:\\x", "C:\\x", "c:\\x", true, ""},
		{"\\\\?\\c:/x/", "C:\\x", "c:\\x", true, ""},
		{"\\\\?\\UNC\\srv\\share\\x", "\\\\srv\\share\\x", "\\\\srv\\share\\x", true, ""},
		{"\\\\?\\unc\\srv\\share", "\\\\srv\\share", "\\\\srv\\share", true, ""},
		{"\\\\?\\Unc\\srv\\share", "\\\\srv\\share", "\\\\srv\\share", true, ""},
		{"\\\\srv/share//x/", "\\\\srv\\share\\x", "\\\\srv\\share\\x", true, ""},
		{"\\\\srv\\Share\\X", "\\\\srv\\Share\\X", "\\\\srv\\share\\x", true, ""},
		{"/srv/a\\b", "/srv/a\\b", "/srv/a\\b", false, ""},
		{"/srv/proj/", "/srv/proj/", "/srv/proj/", false, ""},
		{"//srv/share", "//srv/share", "//srv/share", false, ""},
		{"C:", "C:", "C:", false, ""},
		{"C:relative", "C:relative", "C:relative", false, ""},
		{"\\\\", "\\\\", "\\\\", false, ""},
		{"", "", "", false, ""},
		{"\\\\?\\", "\\\\?\\", "\\\\?\\", false, ""},
		{"\\\\?\\UNC\\", "\\\\?\\UNC\\", "\\\\?\\UNC\\", false, ""},
		{"\\\\?\\Volume{1}\\x", "\\\\?\\Volume{1}\\x", "\\\\?\\Volume{1}\\x", false, ""},
		{"1:\\x", "1:\\x", "1:\\x", false, ""},
		{"C:\\USERS\\sean\\Demo\\", "C:\\USERS\\sean\\Demo", "c:\\users\\sean\\demo", true, ""},
		{"\\\\?\\UNC\\Srv\\Share", "\\\\Srv\\Share", "\\\\srv\\share", true, ""},
		{"/srv/Proj", "/srv/Proj", "/srv/Proj", false, ""},
		{"D:\\Caf\U000000C9\\\U000000C0", "D:\\Caf\U000000C9\\\U000000C0", "d:\\caf\U000000C9\\\U000000C0", true, "d:\\caf\U000000E9\\\U000000E0"},
		{"\\\\\\\\x", "\\\\x", "\\\\x", true, ""},
		{"\\\\.\\pipe\\x", "\\\\.\\pipe\\x", "\\\\.\\pipe\\x", true, ""},
		{"\\/x", "\\/x", "\\/x", false, ""},
		{"z:x\\y", "z:x\\y", "z:x\\y", false, ""},
		{"C:\\a/b\\c", "C:\\a\\b\\c", "c:\\a\\b\\c", true, ""},
		{"\\\\?/C:/x", "\\\\?/C:/x", "\\\\?/C:/x", false, ""},
		{"\\\\?/UNC/srv/share", "\\\\?/UNC/srv/share", "\\\\?/UNC/srv/share", false, ""},
		{"\\\\?/Ab", "\\\\?/Ab", "\\\\?/Ab", false, ""},
		{"\\\\?/0", "\\\\?/0", "\\\\?/0", false, ""},
		{"\\\\?", "\\\\?", "\\\\?", true, ""},
		{"\\\\?\\\\\\?\\C:\\x", "\\\\?\\\\\\?\\C:\\x", "\\\\?\\\\\\?\\C:\\x", false, ""},
		{"\\\\?\\UNC\\?\\C:\\x", "\\\\?\\UNC\\?\\C:\\x", "\\\\?\\UNC\\?\\C:\\x", false, ""},
		{"\\\\?\\UNC\\?\\UNC\\a\\b", "\\\\?\\UNC\\?\\UNC\\a\\b", "\\\\?\\UNC\\?\\UNC\\a\\b", false, ""},
		{"\\\\?\\Ab", "\\\\?\\Ab", "\\\\?\\Ab", false, ""},
	}
}

// compactCase is a text and its compact form.
type compactCase struct {
	input, want string
}

// compactCases returns the compact cases.
func compactCases() []compactCase {
	return []compactCase{
		{"", ""},
		{"  a  ", "a"},
		{"\U000000A0\U00002028a b\U00003000", "a b"},
		{"\n\tline one\n  line two\n", "line one\n  line two"},
		{"\x1c\x1fa\x1e", "\x1c\x1fa\x1e"},
		{"\U0000200Ba\U0000FEFF", "\U0000200Ba\U0000FEFF"},
		{"a\x00", "a\x00"},
		{"\U00000085x\U00000085", "x"},
	}
}
