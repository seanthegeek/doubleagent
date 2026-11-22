package timeline

import (
	"encoding/json"
	"io"
)

// WriteRows writes rows to w as timeline.jsonl lines, one write per row.
func WriteRows(w io.Writer, rows []Row) error {
	encoder := newLineEncoder(w)
	for i := range rows {
		if err := encoder.Encode(&rows[i]); err != nil {
			return err
		}
	}
	return nil
}

// WriteSessions writes summaries to w as sessions.jsonl lines, one write
// per summary. A summary without models has an empty list, never null.
func WriteSessions(w io.Writer, summaries []SessionSummary) error {
	encoder := newLineEncoder(w)
	for i := range summaries {
		summary := summaries[i]
		if summary.Models == nil {
			summary.Models = []string{}
		}
		if err := encoder.Encode(&summary); err != nil {
			return err
		}
	}
	return nil
}

// newLineEncoder returns an encoder writing one compact record per line,
// UTF-8 kept and HTML characters not escaped; encoding/json still escapes
// U+2028 and U+2029, which some readers take as line breaks.
func newLineEncoder(w io.Writer) *json.Encoder {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder
}
