package parserkit

import "strings"

// Composite is a tracked state written as fixed pieces joined in order,
// such as a permission mode of several fields, that tells whether the
// joined text differs from the text last recorded without joining it on
// each event. An event sets only the pieces it carries (Set), so a piece
// copied from an earlier event, however long, is not read again; the
// answer is that of comparing the joined texts.
type Composite struct {
	pieces, recorded []string
	// differs marks the pieces that differ from their recorded piece.
	differs []bool
	held    bool
	// joinedKnown is set when the joined texts were compared since the
	// last change, and joinedSame holds the answer.
	joinedKnown, joinedSame bool
	work                    Work
}

// NewComposite returns a Composite of n empty pieces with nothing recorded,
// counting in work the bytes of joined texts it compares.
func NewComposite(n int, work Work) *Composite {
	return &Composite{pieces: make([]string, n), recorded: make([]string, n), differs: make([]bool, n),
		work: work}
}

// Set makes piece the i-th piece. A piece equal to the one it replaces
// changes nothing; comparing them reads at most the new piece.
func (c *Composite) Set(i int, piece string) {
	if c.pieces[i] == piece {
		return
	}
	c.pieces[i] = piece
	c.differs[i] = !c.held || piece != c.recorded[i]
	c.joinedKnown = false
}

// Recorded reports whether a text was recorded.
func (c *Composite) Recorded() bool {
	return c.held
}

// Changed reports whether the joined pieces differ from the text last
// recorded, true when none was. Pieces that differ but keep their
// lengths keep every other piece in its place, so the texts differ; only
// pieces whose lengths shift the rest are joined and compared.
func (c *Composite) Changed() bool {
	if !c.held {
		return true
	}
	differ, aligned, length, recordedLength := false, true, 0, 0
	for i, piece := range c.pieces {
		length, recordedLength = length+len(piece), recordedLength+len(c.recorded[i])
		if c.differs[i] {
			differ = true
			aligned = aligned && len(piece) == len(c.recorded[i])
		}
	}
	switch {
	case !differ:
		return false
	case length != recordedLength || aligned:
		return true
	case !c.joinedKnown:
		c.work.Add("composite bytes compared", 2*length)
		c.joinedSame = strings.Join(c.pieces, "") == strings.Join(c.recorded, "")
		c.joinedKnown = true
	}
	return !c.joinedSame
}

// Record records the joined pieces as the text later changes are told
// from.
func (c *Composite) Record() {
	copy(c.recorded, c.pieces)
	clear(c.differs)
	c.held = true
	c.joinedKnown = false
}
