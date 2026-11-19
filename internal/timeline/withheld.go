package timeline

import "github.com/seanthegeek/doubleagent/internal/secrets"

// Withheld returns the row as timeline.jsonl writes it: each of its strings
// through recorder.Text with the secrets of the parse that gave it, so a
// run that withholds secrets replaces them in every field. A nil recorder
// returns the row as it is.
func (r Row) Withheld(recorder *secrets.Recorder) Row {
	if recorder == nil {
		return r
	}
	for _, field := range []*string{&r.TimestampUTC, &r.Host, &r.User, &r.Agent, &r.SessionID, &r.ProjectPath,
		&r.GitBranch, &r.TurnType, &r.Model, &r.ToolName, &r.ToolUseID, &r.Text, &r.SourceFile, &r.Recovered} {
		*field = recorder.Text(*field, r.Secrets)
	}
	return r
}

// Withheld returns the summary as sessions.jsonl writes it: each of its
// strings through recorder.Text with the secrets of the parses that gave
// the session's rows. A nil recorder returns the summary as it is.
func (s SessionSummary) Withheld(recorder *secrets.Recorder) SessionSummary {
	if recorder == nil {
		return s
	}
	for _, field := range []*string{&s.Host, &s.User, &s.Agent, &s.SessionID, &s.ProjectPath,
		&s.FirstTimestampUTC, &s.LastTimestampUTC, &s.SourceFile, &s.Synced, &s.Relayed, &s.Origin,
		&s.Recovered} {
		*field = recorder.Text(*field, s.Secrets...)
	}
	models := make([]string, len(s.Models))
	for i, model := range s.Models {
		models[i] = recorder.Text(model, s.Secrets...)
	}
	s.Models = models
	return s
}
