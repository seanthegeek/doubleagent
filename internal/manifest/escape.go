package manifest

const hexDigits = "0123456789abcdef"

// EscapeJSON returns text as the body of a JSON string. It escapes `"` and
// `\`, writes \b, \t, \n, \f and \r by name and every other byte below 0x20
// as \u00xx in lower-case hex. Every other byte, invalid UTF-8 included,
// passes through, since a path is bytes.
func EscapeJSON(text string) string {
	return string(appendEscaped(nil, text))
}

// appendEscaped appends EscapeJSON(text) to out.
func appendEscaped(out []byte, text string) []byte {
	for i := 0; i < len(text); i++ {
		char := text[i]
		switch {
		case char == '"' || char == '\\':
			out = append(out, '\\', char)
		case char == '\b':
			out = append(out, '\\', 'b')
		case char == '\t':
			out = append(out, '\\', 't')
		case char == '\n':
			out = append(out, '\\', 'n')
		case char == '\f':
			out = append(out, '\\', 'f')
		case char == '\r':
			out = append(out, '\\', 'r')
		case char < 0x20:
			out = append(out, '\\', 'u', '0', '0', hexDigits[char>>4], hexDigits[char&0xf])
		default:
			out = append(out, char)
		}
	}
	return out
}
