package analyze

import "strings"

// appendCSVRow appends one CSV row to out as Python's csv module writes it
// in its default dialect: fields separated by commas, a field holding a
// comma, a quote, a CR or an LF put in quotes with each quote doubled, any
// other field as it is (a leading space or an embedded tab included), and
// the row ended by CR LF.
func appendCSVRow(out []byte, fields []string) []byte {
	for i, field := range fields {
		if i > 0 {
			out = append(out, ',')
		}
		if !strings.ContainsAny(field, ",\"\r\n") {
			out = append(out, field...)
			continue
		}
		out = append(out, '"')
		out = append(out, strings.ReplaceAll(field, `"`, `""`)...)
		out = append(out, '"')
	}
	if len(fields) == 1 && fields[0] == "" {
		out = append(out, `""`...)
	}
	return append(out, '\r', '\n')
}
