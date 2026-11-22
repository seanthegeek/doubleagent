//go:build unix

package imagetest

import "testing"

// unixSetups names every Unix fixture builder, as the cases use them.
func unixSetups() map[string]func(Image, testing.TB) {
	return map[string]func(Image, testing.TB){
		"env scan once":                Image.EnvScanOnce,
		"relative project":             Image.RelativeProject,
		"unreadable home":              Image.UnreadableHome,
		"unreadable case variant":      Image.UnreadableCaseVariant,
		"unreadable volume":            Image.UnreadableVolume,
		"docker desktop":               Image.DockerDesktop,
		"inventory":                    Image.Inventory,
		"unreadable inventory home":    Image.UnreadableInventoryHome,
		"batch root":                   func(img Image, test testing.TB) { img.BatchRoot(test) },
		"bad name":                     func(img Image, test testing.TB) { img.BadName(test) },
		"project out of home":          func(img Image, test testing.TB) { img.ProjectOutOfHome(test, false) },
		"absolute project out of home": func(img Image, test testing.TB) { img.ProjectOutOfHome(test, true) },
		"linked project":               func(img Image, test testing.TB) { img.LinkedProject(test, false) },
		"absolute linked project":      func(img Image, test testing.TB) { img.LinkedProject(test, true) },
		"project hint file":            Image.ProjectHintFile,
		"project of another user":      Image.ProjectOfAnotherUser,
		"project on the host":          Image.ProjectOnTheHost,
		"framework volumes":            Image.FrameworkVolumes,
		"pattern root":                 func(img Image, test testing.TB) { img.PatternRoot(test) },
		"msix root":                    func(img Image, test testing.TB) { img.MsixRoot(test) },
		"linked source intact":         Image.LinkedSourceIntact,
	}
}

func TestEveryFixtureBuilderLeavesTheImageAsBuilt(t *testing.T) {
	img := Build(t, t.TempDir())
	built, err := Read(img.Work)
	if err != nil {
		t.Fatal(err)
	}
	for name, setup := range unixSetups() {
		var during []Entry
		t.Run(name+" changes the image and then puts it back", func(t *testing.T) {
			setup(img, t)
			during, _ = Read(img.Work)
		})
		after, err := Read(img.Work)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range differences(built, after, false, true) {
			t.Errorf("after %s: %s", name, line)
		}
		if name != "linked source intact" && len(differences(built, during, false, true)) == 0 {
			t.Errorf("%s changed nothing in the image", name)
		}
	}
}
