package parserkit

// Space is the body of a regular-expression character class holding the
// characters unicode.IsSpace reports, the whitespace strings.TrimSpace and
// strings.Fields use: ASCII whitespace, U+0085, U+00A0 and the Unicode
// spaces and separators. Go's \s is ASCII only, so a pattern that splits
// or trims at any whitespace writes `[` + Space + `]` for it and
// `[^` + Space + `]` for its complement.
const Space = `\t\n\v\f\r \x85\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`
