//go:build parity

package replay

import (
	"regexp"
	"slices"
	"strings"
)

// side names which text a shape is read from: the recording, the binary's
// output, or either, where both sides word the detail through a library of
// their own and nothing tells one wording from the other.
type side int

const (
	recordedSide side = iota + 1
	binarySide
	eitherSide
)

// libraryShape is one library or system error detail of difference 102:
// the pattern of the detail on the sides it is read from, and the form
// that replaces it, a template of the pattern's groups. Each pattern is
// anchored on the reader's own wording or on the fixed text of the row
// around it, so it takes the detail and nothing beside it; a detail inside
// a line of JSON stops at a quote.
type libraryShape struct {
	pattern     *regexp.Regexp
	replacement string
	side        side
}

// rowEnd ends a detail that runs to the end of a row's text: a quote in a
// line of JSON, the end of a line, or the end of the text. A closing
// parenthesis before it is the row's own and is kept.
const rowEnd = `(\)?)("|\n|\z)`

// pythonJSONMessages are the messages of Python's JSON decoder, before its
// position.
func pythonJSONMessages() []string {
	return []string{`Expecting value`, `Expecting ',' delimiter`, `Expecting ':' delimiter`,
		`Expecting property name enclosed in double quotes`, `Unterminated string starting at`,
		`Invalid control character '[^'\n]{1,12}' at`, `Invalid \\{1,2}escape: '[^'\n]{1,8}'`,
		`Invalid \\{1,2}uXXXX escape`, `Extra data`, `Illegal trailing comma before end of (?:object|array)`,
		`Unexpected UTF-8 BOM \(decode using utf-8-sig\)`}
}

// goJSONDetails are the details encoding/json gives a syntax error, and
// the binary's own for the end of the text and for text after a value.
func goJSONDetails() []string {
	return []string{`invalid character '.{1,12}?' (?:looking for beginning of (?:value|object key string)|` +
		`after (?:array element|object key:value pair|object key|top-level value|decimal point in numeric literal)|` +
		`in (?:string literal|numeric literal|string escape code|\\{1,2}u hexadecimal character escape|` +
		`exponent of numeric literal|literal (?:true|false|null))|exceeded max depth)`,
		`unexpected end of JSON input`, `extra data after the value`}
}

// expatMessages are the messages of the XML reader the recording used.
func expatMessages() []string {
	return []string{"not well-formed (invalid token)", "syntax error", "no element found", "mismatched tag",
		"unclosed token", "partial character", "duplicate attribute", "junk after document element",
		"illegal parameter entity reference", "undefined entity", "recursive entity reference",
		"asynchronous entity", "reference to invalid character number", "reference to binary entity",
		"reference to external entity in attribute", "XML or text declaration not at start of entity",
		"unknown encoding", "encoding specified in XML declaration is incorrect", "unclosed CDATA section",
		"error in processing external entity reference", "document is not standalone",
		"unexpected parser state - please send a bug report", "entity declared in parameter entity",
		"requested feature requires XML_DTD support in Expat", "cannot change setting once parsing has begun",
		"unbound prefix", "must not undeclare prefix", "incomplete markup in parameter entity",
		"XML declaration not well-formed", "text declaration not well-formed", "illegal character(s) in public id",
		"parser suspended", "parser not suspended", "parsing aborted", "parsing finished",
		"cannot suspend in external parameter entity"}
}

// goXMLMessages are the messages of encoding/xml's syntax errors, each a
// pattern, and the reader's own for a document with no root element or
// with content outside it.
func goXMLMessages() []string {
	name := `[^"<>\n]*`
	return []string{`attribute name without = in element`, `element <` + name + `> closed by </` + name + `>`,
		`element <` + name + `> in space ` + name + ` closed by ` + name + ` in space ` + name,
		`expected (?:/> in element|attribute name in element|element name after </?|target name after <\?)`,
		`invalid (?:<!\[ sequence|character entity &` + name + `;?|characters between </` + name + ` and >|` +
			`sequence <!- not part of <!--|UTF-8|XML name: ` + name + `)`,
		`unescaped (?:\]\]> not in CDATA section|< inside quoted string)`, `unexpected end element </` + name + `>`,
		`unexpected EOF(?: in CDATA section)?`, `unquoted or missing attribute value in element`,
		`illegal character code U\+[0-9A-F]{4,6}`, `no root element`, `content outside the root element`,
		`a second root element`, `text outside the root element`}
}

// tomlMessages start the messages of the TOML reader the recording used.
func tomlMessages() []string {
	return []string{`An object other than list found behind this key`, `Cannot declare`,
		`Cannot mutate immutable namespace`, `Cannot overwrite a value`, `Cannot redefine namespace`,
		`Duplicate inline table key`, `Escaped character is not a Unicode scalar value`, `Expected`,
		`Found invalid character`, `Illegal character`, `Invalid date or datetime`, `Invalid hex value`,
		`Invalid initial character for a key part`, `Invalid statement`, `Invalid value`,
		`There is no nest behind this key`, `Unclosed array`, `Unclosed inline table`,
		`Unescaped '\\{1,2}' in a string`, `Unterminated string`}
}

// quotedAlternatives returns the strings as one alternation of literals.
func quotedAlternatives(texts []string) string {
	quoted := make([]string, len(texts))
	for i, text := range texts {
		quoted[i] = regexp.QuoteMeta(text)
	}
	return "(?:" + strings.Join(quoted, "|") + ")"
}

// xmlFrame is the fixed text of a row of an XML document that did not
// parse, before the reader's message: a cody_history.xml with the chats
// read before the damage, or a Tabby history.
const xmlFrame = `((?:cody_history\.xml did not parse; read [0-9]+ chat\(s\)|not a readable XML document) \()`

// shapeOf returns the shape of difference 102 that rewrites what pattern finds
// as replacement in the text one side's reader wrote.
func shapeOf(pattern, replacement string, side side) libraryShape {
	return libraryShape{regexp.MustCompile(pattern), replacement, side}
}

// textReaderShapes returns the shapes of difference 102 for the readers of JSON, of
// UTF-8 text and of command lines. A shape on one side only is the wording of
// that side's reader; the other side's reader words the same condition in its
// own shape, which gives the same form.
func textReaderShapes() []libraryShape {
	return []libraryShape{
		shapeOf(`(?:JSONDecodeError: )?(?:`+strings.Join(pythonJSONMessages(), "|")+`): line ([0-9]+) column ([0-9]+) `+
			`\(char [0-9]+\)`, "{json error}", recordedSide),
		shapeOf(`invalid JSON at line [0-9]+, column [0-9]+: (?:`+strings.Join(goJSONDetails(), "|")+`)`,
			"{json error}", binarySide),
		shapeOf(`\(ValueError: not a JSON object\)`, "({thread not an object})", recordedSide),
		shapeOf(`\(the thread is not a JSON object\)`, "({thread not an object})", binarySide),
		shapeOf(`created_git_worktrees: KeyError: 'created_at_seconds'\)`,
			"created_git_worktrees: {no creation time})", recordedSide),
		shapeOf(`created_git_worktrees: no created_at_seconds field\)`,
			"created_git_worktrees: {no creation time})", binarySide),
		shapeOf(`'utf-8' codec can't decode (?:byte 0x[0-9a-f]{2} in position [0-9]+|bytes in position [0-9]+-[0-9]+): `+
			`(?:invalid start byte|invalid continuation byte|unexpected end of data)`, "{utf-8 error}", recordedSide),
		shapeOf(`not valid UTF-8 at byte offset [0-9]+`, "{utf-8 error}", binarySide),
		shapeOf(`unparseable line \(No closing quotation\)`, "unparseable line ({unclosed quote})", recordedSide),
		shapeOf(`unparseable line \(a quote is not closed\)`, "unparseable line ({unclosed quote})", binarySide),
	}
}

// markupReaderShapes returns the shapes of difference 102 for the readers of XML
// and TOML.
func markupReaderShapes() []libraryShape {
	return []libraryShape{
		shapeOf(xmlFrame+quotedAlternatives(expatMessages())+`: line [0-9]+, column [0-9]+\)`+rowEnd,
			"${1}{xml error})${2}${3}", recordedSide),
		shapeOf(xmlFrame+`(?:`+strings.Join(goXMLMessages(), "|")+`): line [0-9]+, column [0-9]+\)`+rowEnd,
			"${1}{xml error})${2}${3}", binarySide),
		shapeOf(`(?:`+strings.Join(tomlMessages(), "|")+`)[^"\n]*? \(at (?:line [0-9]+, column [0-9]+|end of document)\)`+
			rowEnd, "{toml error}${1}${2}", recordedSide),
		shapeOf(`invalid TOML at line [0-9]+, column [0-9]+: [^"\n]*?`+rowEnd, "{toml error}${1}${2}", binarySide),
	}
}

// archiveReaderShapes returns the shapes of difference 102 for the readers of
// zstd, zip, base64 and compressed data.
func archiveReaderShapes() []libraryShape {
	return []libraryShape{
		shapeOf(`zstd: file ends inside a zstd frame`+rowEnd, "zstd: {zstd cut off}${1}${2}", recordedSide),
		shapeOf(`zstd: the file ends inside a zstd frame`+rowEnd, "zstd: {zstd cut off}${1}${2}", binarySide),
		shapeOf(`zstd: zstd decompressor error: [^"\n]*?`+rowEnd, "zstd: {zstd error}${1}${2}", recordedSide),
		shapeOf(`zstd: (?:corrupt data \([^()"\n]*\)|a frame needs a window larger than the decoder allows)`+rowEnd,
			"zstd: {zstd error}${1}${2}", binarySide),
		shapeOf(`is not a zip archive \(File is not a zip file\)`, "is not a zip archive ({not a zip})", recordedSide),
		shapeOf(`is not a zip archive \(not a valid zip archive\)`, "is not a zip archive ({not a zip})", binarySide),
		shapeOf(`is not base64 \((?:Only base64 data is allowed|Incorrect padding|Excess data after padding|`+
			`Leading padding not allowed|Discontinuous padding not allowed|Invalid base64-encoded string: [^()"\n]*)\)`,
			"is not base64 ({base64 error})", recordedSide),
		shapeOf(`is not base64 \(invalid base64 data at byte [0-9]+\)`, "is not base64 ({base64 error})",
			binarySide),
		shapeOf(`(could not be read \()Error -?[0-9]+ while decompressing data: [^()"\n]*\)`,
			"${1}{damaged compressed data})", recordedSide),
		shapeOf(`(could not be read \()damaged compressed data at byte [0-9]+\)`, "${1}{damaged compressed data})",
			binarySide),
		shapeOf(`(could not be read \()Bad CRC-32 for file '[^'"\n]*'\)`, "${1}{checksum mismatch})", recordedSide),
		shapeOf(`(could not be read \()checksum mismatch\)`, "${1}{checksum mismatch})", binarySide),
		shapeOf(`(could not be read \()Compressed file ended before the end-of-stream marker was reached\)`,
			"${1}{data cut off})", recordedSide),
		shapeOf(`(could not be read \()data cut off\)`, "${1}{data cut off})", binarySide),
		shapeOf(`(member [^"\n]* could not be read \()Bad magic number for file header\)`, "${1}{not a zip member})",
			recordedSide),
		shapeOf(`(member [^"\n]* could not be read \()not a valid zip archive\)`, "${1}{not a zip member})", binarySide),
		shapeOf(`(member [^"\n]* could not be read \()negative seek value -[0-9]+\)`, "${1}{offset before the start})",
			recordedSide),
		shapeOf(`(member [^"\n]* could not be read \()header placed before the start of the archive\)`,
			"${1}{offset before the start})", binarySide),
	}
}

// sqliteReaderShapes returns the shapes of difference 102 for the readers of SQLite
// databases, each written the same on either side.
func sqliteReaderShapes() []libraryShape {
	return []libraryShape{
		shapeOf(`SQLite error: [^"\n]*?`+rowEnd, "SQLite error: {sqlite error}${1}${2}", eitherSide),
		shapeOf(`database could not be read: [^"\n]*?`+rowEnd, "database could not be read: {sqlite error}${1}${2}",
			eitherSide),
		shapeOf(`state\.vscdb unreadable: [^"\n]*?`+rowEnd, "state.vscdb unreadable: {sqlite error}${1}${2}",
			eitherSide),
		shapeOf(`unreadable \([^()"\n]*\)`, "unreadable ({sqlite error})", eitherSide),
	}
}

// libraryText returns the rewrite of difference 102 for one side: the
// shapes read from that side rewritten in each line's error text, from
// its marker to the line's end, and the mutation path and system message
// in their compared forms.
func libraryText(from side) func(text string) (string, int) {
	shapes := slices.Concat(textReaderShapes(), markupReaderShapes(), archiveReaderShapes(), sqliteReaderShapes())
	return func(text string) (string, int) {
		lines := strings.SplitAfter(text, "\n")
		total := 0
		for i, line := range lines {
			location := errorMarker.FindStringIndex(line)
			if location == nil {
				continue
			}
			rest := line[location[1]:]
			for _, shape := range shapes {
				if shape.side != from && shape.side != eitherSide {
					continue
				}
				found := len(shape.pattern.FindAllStringIndex(rest, -1))
				if found > 0 {
					rest = shape.pattern.ReplaceAllString(rest, shape.replacement)
					total += found
				}
			}
			var found int
			rest, found = mutationPathText(rest, from)
			total += found
			if from == recordedSide {
				rest, found = systemReasonText(rest)
				total += found
			}
			lines[i] = line[:location[1]] + rest
		}
		return strings.Join(lines, ""), total
	}
}

// Paths of a mutation not applied: the recording writes the path as
// Python's list of strings and integers, the binary as a JSON array, its
// quotes escaped inside a line of JSON.
var (
	pythonMutationPath = regexp.MustCompile(`mutation not applied \(path \[((?:'[^'"\\\n]*'|[0-9]+)` +
		`(?:, (?:'[^'"\\\n]*'|[0-9]+))*)\] not found\)`)
	jsonMutationPath = regexp.MustCompile(`mutation not applied \(path \[((?:\\?"[^'"\\\n]*\\?"|[0-9]+)` +
		`(?:, ?(?:\\?"[^'"\\\n]*\\?"|[0-9]+))*)\] not found\)`)
	pathElement = regexp.MustCompile(`'[^'"\\\n]*'|\\?"[^'"\\\n]*\\?"|[0-9]+`)
)

// mutationPathText writes each path of a mutation not applied, in the
// form of the given side, as the list of its elements, each string in
// angle brackets, and returns the count.
func mutationPathText(text string, from side) (string, int) {
	pattern := pythonMutationPath
	if from == binarySide {
		pattern = jsonMutationPath
	}
	found := 0
	text = pattern.ReplaceAllStringFunc(text, func(match string) string {
		found++
		var elements []string
		for _, element := range pathElement.FindAllString(pattern.FindStringSubmatch(match)[1], -1) {
			if trimmed := strings.Trim(element, `'"\`); trimmed != element {
				element = "<" + trimmed + ">"
			}
			elements = append(elements, element)
		}
		return "mutation not applied (path {mutation path " + strings.Join(elements, " ") + "} not found)"
	})
	return text, found
}

// systemReason is a reason the recording gives for a file it could not
// read, as Python's strerror words the six conditions the binary names in
// lower case, at the end of a parenthesis of could not be read.
var systemReason = regexp.MustCompile(`could not be read \((Permission denied|No such file or directory|` +
	`Is a directory|Not a directory|Too many levels of symbolic links|File name too long)\)`)

// systemReasonText writes the first letter of each reason of systemReason
// in lower case, the binary's own form, which is then compared as written,
// and returns the count.
func systemReasonText(text string) (string, int) {
	found := 0
	text = systemReason.ReplaceAllStringFunc(text, func(match string) string {
		found++
		head, reason, _ := strings.Cut(match, "(")
		return head + "(" + strings.ToLower(reason[:1]) + reason[1:]
	})
	return text, found
}
