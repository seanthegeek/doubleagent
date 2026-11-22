//go:build parity && unix

package parity

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// caseBuilders names the imagetest builder of each Unix case that builds
// fixtures before its run.
func caseBuilders() map[string]func(imagetest.Image, testing.TB) {
	msix := func(img imagetest.Image, test testing.TB) { img.MsixRoot(test) }
	return map[string]func(imagetest.Image, testing.TB){
		"root486":            func(img imagetest.Image, test testing.TB) { img.PatternRoot(test) },
		"projout503":         func(img imagetest.Image, test testing.TB) { img.ProjectOutOfHome(test, false) },
		"projoutabs503":      func(img imagetest.Image, test testing.TB) { img.ProjectOutOfHome(test, true) },
		"projself503":        func(img imagetest.Image, test testing.TB) { img.LinkedProject(test, false) },
		"projselfabs503":     func(img imagetest.Image, test testing.TB) { img.LinkedProject(test, true) },
		"projhint503":        imagetest.Image.ProjectHintFile,
		"projhost503":        imagetest.Image.ProjectOnTheHost,
		"projother503":       imagetest.Image.ProjectOfAnotherUser,
		"envscanonce":        imagetest.Image.EnvScanOnce,
		"dockerperm":         imagetest.Image.UnreadableVolume,
		"desktop":            imagetest.Image.DockerDesktop,
		"fwvolumes":          imagetest.Image.FrameworkVolumes,
		"fwvolumesnosecrets": imagetest.Image.FrameworkVolumes,
		"projargs":           imagetest.Image.RelativeProject,
		"readperm":           imagetest.Image.UnreadableHome,
		"fold32":             imagetest.Image.UnreadableCaseVariant,
		"badname51":          func(img imagetest.Image, test testing.TB) { img.BadName(test) },
		"batch71":            func(img imagetest.Image, test testing.TB) { img.BatchRoot(test) },
		"store73":            msix,
		"nosecrets73":        msix,
		"full73":             msix,
		"inventory":          imagetest.Image.Inventory,
		"inventory-o":        imagetest.Image.Inventory,
		"inventory-full":     imagetest.Image.Inventory,
		"inventory-u":        imagetest.Image.Inventory,
		"inventory-perm":     imagetest.Image.UnreadableInventoryHome,
	}
}

// caseChecks names the imagetest check each Unix case with an after step
// runs after its run.
func caseChecks() map[string]func(imagetest.Image, testing.TB) {
	return map[string]func(imagetest.Image, testing.TB){"linkproj497": imagetest.Image.LinkedSourceIntact}
}
