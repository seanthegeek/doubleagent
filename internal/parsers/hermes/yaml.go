package hermes

import (
	"errors"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// quotedScalar reads a scalar that starts with a quote: only a comment may
// follow its closing quote.
func quotedScalar(value string) (string, bool) {
	quote, i := value[0], 1
	for i < len(value) {
		escaped := quote == '"' && value[i] == '\\' ||
			quote == '\'' && strings.HasPrefix(value[i:], "''")
		switch {
		case escaped:
			i += 2
			continue
		case value[i] == quote:
		default:
			i++
			continue
		}
		break
	}
	if i >= len(value) {
		return "", false
	}
	if rest := strings.Trim(value[i+1:], textlines.SpaceOrTab); rest != "" && !strings.HasPrefix(rest, "#") {
		return "", false
	}
	if quote == '\'' {
		return strings.ReplaceAll(value[1:i], "''", "'"), true
	}
	decoded, err := jsonvalue.DecodeString(value[:i+1])
	if err != nil || !decoded.IsString() {
		return "", false
	}
	return decoded.StringValue(), true
}

// yamlScalar returns a one-line YAML scalar as a string, or false for
// anything else: a double-quoted scalar (JSON's escapes), a single-quoted
// one (a doubled single quote for a quote) and a plain one (a trailing " #" comment removed)
// are read; a mapping, a flow collection, an alias, a block scalar or an
// unterminated quote is not.
func (p *Parser) yamlScalar(value string) (string, bool) {
	if strings.HasPrefix(value, "'") || strings.HasPrefix(value, `"`) {
		return quotedScalar(value)
	}
	plain := value
	if found := p.yamlComment.FindStringIndex(value); found != nil {
		plain = value[:found[0]]
	}
	plain = strings.TrimRight(plain, textlines.SpaceOrTab)
	if plain == "" || strings.IndexByte("[]{}&*!|>%@`#,", plain[0]) >= 0 ||
		p.yamlMapping.MatchString(plain) || strings.HasSuffix(plain, ":") {
		return "", false
	}
	return plain, true
}

// listEntry is one string item of a YAML block list and its line.
type listEntry struct {
	line  int
	value string
}

// readFlatList returns the strings of the top-level YAML block list key,
// with their lines, and an error naming the first line that is not a
// "- <scalar>" item of the list. An absent key, "key:" with no items and
// "key: []" give no entries.
func (p *Parser) readFlatList(lines []string, key string) ([]listEntry, error) {
	start := 0
	for i, line := range lines {
		if !strings.HasPrefix(line, key+":") {
			continue
		}
		start = i + 1
		rest := line[len(key)+1:]
		if found := p.yamlComment.FindStringIndex(rest); found != nil {
			rest = rest[:found[0]]
		}
		switch strings.Trim(rest, textlines.SpaceOrTab) {
		case "", "[]", "null", "~":
		default:
			return nil, errors.New("line " + strconv.Itoa(start) + ": " + key + " is not a block list")
		}
		break
	}
	if start == 0 {
		return nil, nil
	}
	var entries []listEntry
	for number := start + 1; number <= len(lines); number++ {
		line := lines[number-1]
		stripped := strings.Trim(line, textlines.SpaceOrTab)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		if strings.IndexByte(textlines.SpaceOrTab, line[0]) < 0 && !strings.HasPrefix(stripped, "-") {
			break
		}
		value, ok := "", false
		if strings.HasPrefix(stripped, "- ") {
			value, ok = p.yamlScalar(strings.Trim(stripped[2:], textlines.SpaceOrTab))
		}
		if !ok {
			return entries, errors.New("line " + strconv.Itoa(number) + ": not a string item of " + key)
		}
		entries = append(entries, listEntry{number, value})
	}
	return entries, nil
}
