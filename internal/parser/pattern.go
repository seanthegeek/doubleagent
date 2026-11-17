package parser

// PythonSpace is the body of a regular-expression character class holding
// the characters Python's \s matches in a str pattern, the characters
// str.isspace() holds for: ASCII whitespace, U+001C to U+001F and the
// Unicode spaces and separators. Go's \s is ASCII only, so a pattern ported
// from Python writes `[` + PythonSpace + `]` for \s and `[^` + PythonSpace
// + `]` for \S.
const PythonSpace = `\t\n\v\f\r \x1c-\x1f\x85\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`
