package imagetest

import "testing"

// windowsSetups names every Windows fixture builder, as the cases use them.
func windowsSetups() map[string]func(Image, testing.TB) {
	return map[string]func(Image, testing.TB){
		"env scan once":           Image.EnvScanOnce,
		"glob output":             func(img Image, test testing.TB) { img.GlobOutput(test) },
		"docker desktop":          Image.DockerDesktop,
		"relative project":        Image.RelativeProject,
		"inventory":               Image.Inventory,
		"read-only output":        func(img Image, test testing.TB) { img.ReadOnlyOutput(test) },
		"unreadable case variant": Image.UnreadableCaseVariant,
		"link image":              func(img Image, test testing.TB) { img.LinkImage(test) },
		"broken junction home":    func(img Image, test testing.TB) { img.BrokenJunctionHome(test) },
		"non-ASCII root":          func(img Image, test testing.TB) { img.NonASCIIRoot(test) },
		"framework volumes":       Image.FrameworkVolumes,
		"pattern root":            func(img Image, test testing.TB) { img.PatternRoot(test) },
		"msix root":               func(img Image, test testing.TB) { img.MsixRoot(test) },
	}
}

func TestEveryFixtureBuilderLeavesTheImageAsBuilt(t *testing.T) {
	img := Build(t, t.TempDir())
	built, err := Read(img.Work)
	if err != nil {
		t.Fatal(err)
	}
	for name, setup := range windowsSetups() {
		var during []Entry
		t.Run(name+" changes the image and then puts it back", func(t *testing.T) {
			setup(img, t)
			during, _ = Read(img.Work)
		})
		after, err := Read(img.Work)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range differences(built, after, foldsCase(t, img.Work), true) {
			t.Errorf("after %s: %s", name, line)
		}
		if len(differences(built, during, foldsCase(t, img.Work), true)) == 0 {
			t.Errorf("%s changed nothing in the image", name)
		}
	}
}
