//go:build parity && windows

package parity

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// caseBuilders names the imagetest builder of each Windows case that builds
// fixtures before its run.
func caseBuilders() map[string]func(imagetest.Image, testing.TB) {
	msix := func(img imagetest.Image, test testing.TB) { img.MsixRoot(test) }
	return map[string]func(imagetest.Image, testing.TB){
		"root71":             func(img imagetest.Image, test testing.TB) { img.NonASCIIRoot(test) },
		"root486":            func(img imagetest.Image, test testing.TB) { img.PatternRoot(test) },
		"envscanonce":        imagetest.Image.EnvScanOnce,
		"envscanglob":        func(img imagetest.Image, test testing.TB) { img.GlobOutput(test) },
		"desktop":            imagetest.Image.DockerDesktop,
		"fwvolumes":          imagetest.Image.FrameworkVolumes,
		"fwvolumesnosecrets": imagetest.Image.FrameworkVolumes,
		"inventory":          imagetest.Image.Inventory,
		"inventory-o":        imagetest.Image.Inventory,
		"inventory-full":     imagetest.Image.Inventory,
		"inventory-u":        imagetest.Image.Inventory,
		"inventory-nodocker": imagetest.Image.Inventory,
		"inventory-fold32":   imagetest.Image.UnreadableCaseVariant,
		"links":              func(img imagetest.Image, test testing.TB) { img.LinkImage(test) },
		"brokenjunctionhome": func(img imagetest.Image, test testing.TB) { img.BrokenJunctionHome(test) },
		"projargs":           imagetest.Image.RelativeProject,
		"readonly":           func(img imagetest.Image, test testing.TB) { img.ReadOnlyOutput(test) },
		"fold32":             imagetest.Image.UnreadableCaseVariant,
		"store73":            msix,
		"nosecrets73":        msix,
		"full73":             msix,
	}
}
