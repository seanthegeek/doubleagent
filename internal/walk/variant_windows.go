package walk

import (
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// diskSpelling returns path unchanged: on Windows the expander already
// spells each literal segment as its directory holds it (fsmeta.OnDiskName),
// so two spellings that remain are ones the disk could not settle.
func diskSpelling(path, _ string, _ glob.Check, _ *fsmeta.Opener) string {
	return path
}
