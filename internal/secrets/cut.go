package secrets

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/secrets/urlcredentials"
	"github.com/seanthegeek/doubleagent/internal/textset"
)

// CutPoint returns how many leading bytes of value to keep when it is cut
// to at most limit bytes and white space is written after them: limit,
// moved back to the start of a UTF-8 character, then, when the cut would
// end inside a span the cut must keep whole or leave out, back to where
// the span starts, and again until it ends inside none: the URL spans of
// value (cutSpans) and whole, which the caller gives. The answer depends
// on value alone, never on what a run records, so every run cuts a value
// alike; it reads at most twice limit bytes of value.
func CutPoint(value string, limit int, whole [][2]int) int {
	point := min(max(limit, 0), len(value))
	for steps := 1; steps < utf8.UTFMax && point > 0 && point < len(value) && !utf8.RuneStart(value[point]); steps++ {
		point--
	}
	if point == len(value) {
		return point
	}
	return outsideSpans(point, append(cutSpans(value, point, limit), whole...))
}

// cutSpans returns the spans a cut at or before point keeps whole or leaves
// out, so the kept bytes with white space after them hold exactly the URL
// credentials value holds there: each one the output replaces with the
// byte after it (the @ after user information; a query value cut right
// after it would end its token and lose punctuation), and each URL's
// scheme and separator. URLs are read to the end of point's token; when it
// runs more than lookahead bytes past point, the span from its first
// separator's end to the value's end stands for its URLs, since each
// credential of a token comes after its first separator.
func cutSpans(value string, point, lookahead int) [][2]int {
	start := urlcredentials.TokenStart(value, point)
	window := value[:min(len(value), point+lookahead+1)]
	end := urlcredentials.TokenEnd(window, point)
	var spans [][2]int
	if end == len(window) && len(window) < len(value) {
		if separator := firstSeparatorEnd(value[start:point]); separator > 0 {
			spans = append(spans, [2]int{start + separator, len(value)})
		}
		end = start
	}
	text := value[:end]
	if !urlcredentials.MayHold(text) {
		return spans
	}
	identifyURLs(text, func(secret urlcredentials.Credential) {
		spans = append(spans, [2]int{secret.Start, secret.End + 1})
	}, nil)
	return append(spans, urlcredentials.Schemes(text)...)
}

// firstSeparatorEnd returns where the first scheme separator of text, "://"
// or JSON's `:\/\/`, ends, or 0 when text holds none.
func firstSeparatorEnd(text string) int {
	plain, escaped := strings.Index(text, "://"), strings.Index(text, `:\/\/`)
	switch {
	case plain >= 0 && (escaped < 0 || plain < escaped):
		return plain + len("://")
	case escaped >= 0:
		return escaped + len(`:\/\/`)
	}
	return 0
}

// outsideSpans returns point, or, when it lies strictly inside a span, the
// start of the run of spans that overlap around it: the first place at or
// before point that lies inside none, reached by moving back to the start
// of each span it lies inside in turn.
func outsideSpans(point int, spans [][2]int) int {
	slices.SortFunc(spans, func(left, right [2]int) int { return left[0] - right[0] })
	runStart, runEnd := 0, -1
	for _, span := range spans {
		if span[0] >= point {
			break
		}
		if span[0] >= runEnd {
			runStart = span[0]
		}
		runEnd = max(runEnd, span[1])
	}
	if runEnd > point && runStart < point {
		return runStart
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
	if r.cutBefore(cutPlace{written, parse}, value) {
		return
	}
	r.cuts = append(r.cuts, cut{value: value, written: written, kept: kept, parse: parse})
	r.changes++
	r.Collect(value)
}

// cutPlace is what keeps apart the lists of values cut: the text written
// in their place and the record of the parse whose row text holds it (nil
// for a field of a row).
type cutPlace struct {
	written string
	parse   *Found
}

// cutList is the values cut in one place: every one, as a set, and the one
// last met there, which a value carried onto rows in turn with others
// meets again in one comparison.
type cutList struct {
	values map[string]bool
	last   string
}

// cutBefore reports whether value was cut in place before, and adds it to
// the place's list when not. It compares value with the place's last one
// and looks it up in the place's set, each reading value at most once
// (counted as "cut bytes compared"), however many values the place holds.
func (r *Recorder) cutBefore(place cutPlace, value string) bool {
	if r.cutsOf == nil {
		r.cutsOf = map[cutPlace]*cutList{}
	}
	list := r.cutsOf[place]
	if list == nil {
		list = &cutList{values: map[string]bool{}}
		r.cutsOf[place] = list
	} else {
		countWork(r.work, "cut bytes compared", 2*len(value))
		if list.last == value || list.values[value] {
			list.last = value
			return true
		}
	}
	list.values[value], list.last = true, value
	return false
}

// countWork adds n units of the named work to work, when it counts.
func countWork(work textset.Counter, name string, n int) {
	if work != nil {
		work.Add(name, n)
	}
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
