package secrets

import (
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/secrets/urlcredentials"
	"github.com/seanthegeek/doubleagent/internal/textset"
)

// CutPoint returns how many leading bytes of value to keep when it is cut
// to at most limit bytes and white space is written after them, so the
// kept bytes hold exactly the URL credentials the whole value holds there:
// limit, moved back to the start of a UTF-8 character, and back to the
// start of its token (urlcredentials rule 1) when the token's kept part
// holds a scheme separator. A credential needs a separator before it in
// its token, so a kept part without one holds none, and the credentials of
// the tokens before it are found alike in both, the white space ending the
// last one; so a cut never keeps the start of a URL's credentials and
// never makes text read as one. It reads at most limit bytes.
func CutPoint(value string, limit int) int {
	point := min(max(limit, 0), len(value))
	for steps := 1; steps < utf8.UTFMax && point > 0 && point < len(value) && !utf8.RuneStart(value[point]); steps++ {
		point--
	}
	if start := urlcredentials.TokenStart(value, point); urlcredentials.MayHold(value[start:point]) {
		return start
	}
	return point
}

// cut is a value the parser kit cut (Recorder.Cut): the whole value, the
// text written in its place (its first kept bytes, then a mark of the
// cut), and the record of the parse whose row text holds it, nil for a
// field of a row.
type cut struct {
	value, written string
	kept           int
	parse          *Found
}

// cutTails is what replace withholds of the cut values: the end of each
// written text from the start of a secret its kept part ends inside, with
// how many of its bytes are that secret's kept part, and the matcher of
// those ends. It is built again when the run records anything or cuts
// another value (version).
type cutTails struct {
	version int
	tails   map[string]int
	set     *textset.Set
}

// Cut records that the parser kit wrote value as written: its first kept
// bytes (CutPoint), then a mark of the cut that starts with white space.
// inText says the written text stands in the text of the rows of the parse
// running, where the parse's own secrets reach too. The output then
// withholds, wherever the written text stands, the kept part of every
// secret it replaces that the cut ended inside, from the secret's start;
// and every URL credential of value long enough to be replaced everywhere
// is recorded now, since value is written nowhere. Shown output is not
// changed: a nil Recorder records nothing.
func (r *Recorder) Cut(value, written string, kept int, inText bool) {
	if r == nil {
		return
	}
	var parse *Found
	if inText {
		parse = r.current
	}
	for _, earlier := range r.cutsOf[written] {
		if each := r.cuts[earlier]; each.parse == parse && each.value == value {
			return
		}
	}
	if r.cutsOf == nil {
		r.cutsOf = map[string][]int{}
	}
	r.cutsOf[written] = append(r.cutsOf[written], len(r.cuts))
	r.cuts = append(r.cuts, cut{value: value, written: written, kept: kept, parse: parse})
	r.changes++
	r.Collect(value)
}

// CutValue is a value the parser kit cut, as Cut recorded it, for a test
// that checks the output against it: the whole value, the written text,
// its kept bytes, and the record of the parse whose row text holds it (nil
// for a field of a row).
type CutValue struct {
	Value, Written string
	Kept           int
	Parse          *Found
}

// CutValues returns the values Cut recorded, in the order recorded; nil for
// a nil Recorder.
func (r *Recorder) CutValues() []CutValue {
	if r == nil {
		return nil
	}
	values := make([]CutValue, 0, len(r.cuts))
	for _, each := range r.cuts {
		values = append(values, CutValue{each.value, each.written, each.kept, each.parse})
	}
	return values
}

// withhold raises longest at each place text holds the end of a cut
// value's written text that the tails list, by the length of the secret's
// kept part there, so replaceRanges withholds it with whatever else it
// replaces; the mark of the cut after it stays.
func (t *cutTails) withhold(text string, longest []int) {
	if t.set == nil {
		return
	}
	found := make([]int, len(text))
	t.set.Longest(text, found)
	for i, length := range found {
		if length > 0 {
			longest[i] = max(longest[i], t.tails[text[i:i+length]])
		}
	}
}

// cutTails returns the run's cut tails, built again when anything was
// recorded or cut since they were last built.
func (r *Recorder) cutTails(withParts bool) *cutTails {
	if len(r.cuts) == 0 || r.tails.version == r.changes && r.tails.tails != nil {
		return &r.tails
	}
	r.tails = cutTails{version: r.changes, tails: map[string]int{}}
	var texts []string
	for _, each := range r.cuts {
		start := r.secretAcross(each, withParts)
		if start < 0 {
			continue
		}
		tail := each.written[start:]
		if _, held := r.tails.tails[tail]; !held {
			texts = append(texts, tail)
		}
		r.tails.tails[tail] = max(r.tails.tails[tail], each.kept-start)
	}
	if len(texts) > 0 {
		r.tails.set = textset.NewBackward(texts, r.work)
	}
	return &r.tails
}

// secretAcross returns where the first secret the output replaces in the
// whole value that runs past its kept bytes starts, or -1 when none does:
// a secret replaced in every string, and for a value in the text of a
// parse's rows the parse's own secrets too.
func (r *Recorder) secretAcross(each cut, withParts bool) int {
	longest := make([]int, len(each.value))
	if len(r.everywhere.texts[everyString]) > 0 {
		r.everywhere.matcherOf(everyString, withParts, r.work).Longest(each.value, longest)
	}
	if each.parse != nil {
		for _, matcher := range each.parse.appendMatchers(nil, []reach{rowText, parseProblem}, withParts, r.work) {
			matcher.Longest(each.value, longest)
		}
	}
	for i := range each.kept {
		if i+longest[i] > each.kept {
			return i
		}
	}
	return -1
}
