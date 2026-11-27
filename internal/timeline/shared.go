package timeline

import (
	"iter"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// SharedText is a long text that more than one row can name, such as a
// file several records point to: the parser makes one per text, so memory
// holds the text once and two rows name the same text exactly when they
// hold the same SharedText.
type SharedText struct {
	text string
}

// NewSharedText returns the SharedText of text.
func NewSharedText(text string) *SharedText {
	return &SharedText{text: text}
}

// Text returns the shared text.
func (s *SharedText) Text() string {
	return s.text
}

// Pointer returns the text a reference to a shared text is written as
// when another reference holds it in full: holder is the row holding it,
// and sameRow says that row is this one (an earlier reference in the same
// text). The holder comes earlier in the output.
type Pointer func(holder *Row, sameRow bool) string

// SharedRef is one place a row's text names a shared text: the stand-in
// Text holds there until the output writes the row (Text[Start:End]), the
// text written there at the first reference in the output (Full: the
// shared text, trimmed where the row's text begins or ends with it), and
// the pointer written at every later one.
type SharedRef struct {
	Shared     *SharedText
	Start, End int
	Full       string
	Pointer    Pointer
}

// SharedRefs are the references of one row's text, in text order.
type SharedRefs []SharedRef

// TextBuilder builds the text of a row from literal text and shared texts,
// compacted as Compact compacts the text read in full.
type TextBuilder struct {
	pieces []textPiece
}

// textPiece is one piece of a text being built: literal text, or a
// reference with its stand-in.
type textPiece struct {
	text    string
	shared  *SharedText
	standIn string
	pointer Pointer
}

// Write adds literal text.
func (b *TextBuilder) Write(literal string) {
	if literal != "" {
		b.pieces = append(b.pieces, textPiece{text: literal})
	}
}

// Share adds a reference to shared, which the row holds as standIn until
// the output writes it in full or as pointer gives it.
func (b *TextBuilder) Share(shared *SharedText, standIn string, pointer Pointer) {
	b.pieces = append(b.pieces, textPiece{text: shared.text, shared: shared, standIn: standIn,
		pointer: pointer})
}

// Set sets the row's text and references to what was added (Text).
func (b *TextBuilder) Set(row *Row) {
	text := b.Text()
	row.Text, row.Shared = text.Text, text.Shared
}

// Text returns the text and references of what was added, the white space
// at both ends of the full text left out, and empties the builder.
func (b *TextBuilder) Text() RowText {
	pieces := b.pieces
	b.pieces = nil
	for len(pieces) > 0 {
		pieces[0].text = strings.TrimLeftFunc(pieces[0].text, unicode.IsSpace)
		if pieces[0].text != "" {
			break
		}
		pieces = pieces[1:]
	}
	for len(pieces) > 0 {
		last := &pieces[len(pieces)-1]
		last.text = strings.TrimRightFunc(last.text, unicode.IsSpace)
		if last.text != "" {
			break
		}
		pieces = pieces[:len(pieces)-1]
	}
	var text strings.Builder
	var refs SharedRefs
	for _, piece := range pieces {
		if piece.shared == nil {
			text.WriteString(piece.text)
			continue
		}
		start := text.Len()
		text.WriteString(piece.standIn)
		refs = append(refs, SharedRef{Shared: piece.shared, Start: start, End: text.Len(),
			Full: piece.text, Pointer: piece.pointer})
	}
	built := RowText{Text: text.String()}
	if refs != nil {
		built.Shared = &refs
	}
	return built
}

// Pieces yields the row's text as it reads in full, in pieces: the text
// between references and each reference's full text.
func (r *Row) Pieces() iter.Seq[string] {
	return func(yield func(string) bool) {
		if r.Shared == nil {
			yield(r.Text)
			return
		}
		last := 0
		for _, ref := range *r.Shared {
			if !yield(r.Text[last:ref.Start]) || !yield(ref.Full) {
				return
			}
			last = ref.End
		}
		yield(r.Text[last:])
	}
}

// FullText returns the row's text as it reads in full, a new string as
// long as every shared text the row names together.
func (r *Row) FullText() string {
	if r.Shared == nil {
		return r.Text
	}
	var text strings.Builder
	for piece := range r.Pieces() {
		text.WriteString(piece)
	}
	return text.String()
}

// newPiecesReader returns a reader of the runes of pieces.
func newPiecesReader(pieces iter.Seq[string]) *piecesReader {
	return &piecesReader{pieces: slices.Collect(pieces)}
}

// matchesText reports whether pattern matches somewhere in the row's text
// as it reads in full.
func (r *Row) matchesText(pattern *regexp.Regexp) bool {
	if r.Shared == nil {
		return pattern.MatchString(r.Text)
	}
	return pattern.MatchReader(newPiecesReader(r.Pieces()))
}

// piecesReader reads the runes of a text given in pieces; a piece always
// ends at the end of a character.
type piecesReader struct {
	pieces  []string
	current strings.Reader
}

// Matches reports whether pattern matches somewhere in the row's text as
// it reads in full, or in either text of a row set by Parent.Texts. A row
// naming shared texts is read piece by piece, never copied.
func (r *Row) Matches(pattern *regexp.Regexp) bool {
	if r.Group != nil && r.Group.texts != nil {
		for _, text := range r.Group.texts {
			if (&Row{Text: text.Text, Shared: text.Shared}).matchesText(pattern) {
				return true
			}
		}
		return false
	}
	return r.matchesText(pattern)
}

// ReadRune returns the next rune and its length in bytes, or io.EOF once
// every piece is read.
func (p *piecesReader) ReadRune() (rune, int, error) {
	for {
		character, size, err := p.current.ReadRune()
		if err == nil || len(p.pieces) == 0 {
			return character, size, err
		}
		p.current.Reset(p.pieces[0])
		p.pieces = p.pieces[1:]
	}
}

// Resolve writes out the shared texts the rows name, in place, in the
// order of the rows: the first reference to a text holds it in full and
// every later one its pointer naming that row, so a row the filters and
// deduplication left out never holds a text a kept row points to. Each text
// is written in full once, so the output grows with the texts the parsers
// read, not with how often they are named.
func Resolve(rows []Row) {
	holders := map[*SharedText]int{}
	for i := range rows {
		row := &rows[i]
		if row.Shared == nil {
			continue
		}
		var text strings.Builder
		last := 0
		for _, ref := range *row.Shared {
			text.WriteString(row.Text[last:ref.Start])
			if holder, held := holders[ref.Shared]; held {
				text.WriteString(ref.Pointer(&rows[holder], holder == i))
			} else {
				holders[ref.Shared] = i
				text.WriteString(ref.Full)
			}
			last = ref.End
		}
		text.WriteString(row.Text[last:])
		row.Text, row.Shared = Compact(text.String()), nil
	}
}
