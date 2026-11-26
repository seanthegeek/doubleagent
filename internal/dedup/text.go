package dedup

import (
	"encoding/binary"
	"iter"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Texts holds what one deduplication learns about the texts of its rows:
// the digest of each piece of a match text that comes from a shared text
// (a segment), read once however many rows name it, values a hook
// remembers about a shared text (Memo), and the classes of equal match
// texts. Two rows have one text identity exactly when their match texts
// are equal byte for byte.
type Texts struct {
	hasher   hasher
	segments map[segmentKey]*segment
	memos    map[segmentKey]uint64
	groups   map[digest][]*textClass
	classes  int
	// sharedRead counts the bytes of shared texts handed to a segment
	// writer or a Memo computation, the work the cache saves.
	sharedRead int
}

// newTexts returns the empty Texts of a run whose digests use base.
func newTexts(base uint64) *Texts {
	return &Texts{hasher: hasher{base}, segments: map[segmentKey]*segment{},
		memos: map[segmentKey]uint64{}, groups: map[digest][]*textClass{}}
}

// NewTexts returns an empty Texts, for a hook called outside a
// deduplication.
func NewTexts() *Texts {
	return newTexts(randomBase())
}

// SharedRead returns how many bytes of shared texts the run has handed to
// segment writers and Memo computations, the measure of its work there.
func (x *Texts) SharedRead() int {
	return x.sharedRead
}

// fullKey names the text a reference holds in full: two references with
// equal keys hold the same bytes. A reference holds its shared text, maybe
// trimmed of white space at either end (timeline.TextBuilder), so the text
// is known from whether its start was trimmed and its length.
type fullKey struct {
	shared       *timeline.SharedText
	startTrimmed bool
	length       int
}

// keyOf returns the fullKey of a reference.
func keyOf(ref timeline.SharedRef) fullKey {
	whole := ref.Shared.Text()
	trimmed := len(ref.Full) < len(whole) && startsWithSpace(whole) && !startsWithSpace(ref.Full)
	return fullKey{shared: ref.Shared, startTrimmed: trimmed, length: len(ref.Full)}
}

// startsWithSpace reports whether text begins with a unicode.IsSpace
// character.
func startsWithSpace(text string) bool {
	character, _ := utf8.DecodeRuneInString(text)
	return text != "" && unicode.IsSpace(character)
}

// segmentKey names a segment: the full text of a reference, the form that
// rewrites it ("" for as written) and the form's state where it begins.
type segmentKey struct {
	full  fullKey
	form  string
	state uint64
}

// segment is what a segment's writer gave once: its number among the
// run's segments, the digest of what it wrote, and the state it returned.
type segment struct {
	number int
	digest digest
	exit   uint64
}

// SegmentWriter writes the match text of a reference's full text in a
// form, from the state the form was in where it begins, to yield, and
// returns the state where it ends. What it writes and returns must depend
// on nothing but full and state, and it stops when yield returns false;
// the form's name stands for the writer, so one name has one writer.
type SegmentWriter func(full string, state uint64, yield func(string) bool) (exit uint64)

// MatchText is the text a row is matched under, built by its agent's Form
// hook or as the row's text reads in full: literal text, and segments that
// stand for what a shared text becomes, digested once per run.
type MatchText struct {
	texts     *Texts
	parts     []textPart
	digest    digest
	segmented bool
}

// textPart is a piece of a match text: literal text, or a segment with
// the full text it is written from, the state it begins in and its writer.
type textPart struct {
	literal string
	segment *segment
	full    string
	state   uint64
	write   SegmentWriter
}

// NewText returns an empty match text of the run.
func (x *Texts) NewText() *MatchText {
	return &MatchText{texts: x}
}

// Write adds literal text, which the match text holds without copying.
func (t *MatchText) Write(literal string) {
	if literal != "" {
		t.parts = append(t.parts, textPart{literal: literal})
		t.digest = t.texts.hasher.extend(t.digest, literal)
	}
}

// Segment adds what write writes from the full text of ref, in form, from
// state, and returns the state where it ends. The writer runs once per
// run for each full text, form and state.
func (t *MatchText) Segment(ref timeline.SharedRef, form string, state uint64,
	write SegmentWriter) uint64 {
	found := t.texts.segmentOf(segmentKey{keyOf(ref), form, state}, ref.Full, write)
	t.parts = append(t.parts, textPart{segment: found, full: ref.Full, state: state, write: write})
	t.digest = t.texts.hasher.join(t.digest, found.digest)
	t.segmented = true
	return found.exit
}

// WriteRow adds the row's text as it reads in full, each shared text it
// names as written.
func (t *MatchText) WriteRow(row timeline.Row) {
	if row.Shared == nil {
		t.Write(row.Text)
		return
	}
	last := 0
	for _, ref := range *row.Shared {
		t.Write(row.Text[last:ref.Start])
		t.Segment(ref, "", 0, asWritten)
		last = ref.End
	}
	t.Write(row.Text[last:])
}

// asWritten writes a full text as it is.
func asWritten(full string, _ uint64, yield func(string) bool) uint64 {
	yield(full)
	return 0
}

// Memo returns what compute gives for the full text of ref under a name
// and state the hook chooses, computed once per run for each full text,
// name and state; what compute returns must depend on nothing else.
func (x *Texts) Memo(ref timeline.SharedRef, name string, state uint64,
	compute func(full string) uint64) uint64 {
	key := segmentKey{full: keyOf(ref), form: name, state: state}
	if value, ok := x.memos[key]; ok {
		return value
	}
	x.sharedRead += len(ref.Full)
	value := compute(ref.Full)
	x.memos[key] = value
	return value
}

// Memo is Texts.Memo of the text's run.
func (t *MatchText) Memo(ref timeline.SharedRef, name string, state uint64,
	compute func(full string) uint64) uint64 {
	return t.texts.Memo(ref, name, state, compute)
}

// String returns the match text in full, for a test or a message.
func (t *MatchText) String() string {
	var text strings.Builder
	for chunk := range t.chunks() {
		text.WriteString(chunk)
	}
	return text.String()
}

// segmentOf returns the segment of key, running write over full the first
// time.
func (x *Texts) segmentOf(key segmentKey, full string, write SegmentWriter) *segment {
	if found, ok := x.segments[key]; ok {
		return found
	}
	x.sharedRead += len(full)
	var written digest
	exit := write(full, key.state, func(chunk string) bool {
		written = x.hasher.extend(written, chunk)
		return true
	})
	found := &segment{number: len(x.segments), digest: written, exit: exit}
	x.segments[key] = found
	return found
}

// chunks yields the match text in pieces, running each segment's writer
// again.
func (t *MatchText) chunks() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, part := range t.parts {
			if !part.each(t.texts, yield) {
				return
			}
		}
	}
}

// each yields the pieces of a part, a segment's by running its writer
// again (counted in the run's sharedRead), and reports whether yield asked
// for more.
func (p textPart) each(texts *Texts, yield func(string) bool) bool {
	if p.segment == nil {
		return yield(p.literal)
	}
	texts.sharedRead += len(p.full)
	more := true
	p.write(p.full, p.state, func(chunk string) bool {
		more = yield(chunk)
		return more
	})
	return more
}

// arrangement returns the parts of a segmented match text as one string:
// each literal part with its length, each segment by its number. Two
// match texts with one arrangement are equal, since a segment's text
// depends on its key alone.
func (t *MatchText) arrangement() string {
	var out []byte
	for _, part := range t.parts {
		if part.segment != nil {
			out = binary.AppendUvarint(append(out, 's'), uint64(part.segment.number))
			continue
		}
		out = append(binary.AppendUvarint(append(out, 'l'), uint64(len(part.literal))),
			part.literal...)
	}
	return string(out)
}
