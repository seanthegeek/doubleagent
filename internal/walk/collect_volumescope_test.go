//go:build unix

package walk

import (
	"context"
	"reflect"
	"testing"
)

func TestVolumeLinesApplyUnderAVolumeAndNotUnderAHome(t *testing.T) {
	root := fixture{files: []string{"home/u/models/m", "home/u/config",
		"volumes/v/_data/models/m", "volumes/v/_data/config"}}.build(t)
	home, data := root+"/home/u", root+"/volumes/v/_data"
	collector, rows, _ := testCollector(t, root, nil)
	collector.MaxSize, collector.SecretGlobs = 0, nil
	collector.VolumeExcludes, collector.VolumeSecretGlobs = []string{"models"}, []string{"config"}
	ctx := context.Background()
	_, err := collector.CollectPath(ctx, "u", home, "agent", home)
	mustDo(t, err)
	_, err = collector.CollectVolume(ctx, "docker", "agent", data)
	mustDo(t, err)
	type outcome struct{ status, secret string }
	want := map[string]outcome{home + "/config": {"collected", "false"},
		home + "/models/m": {"collected", "false"}, data + "/config": {"collected", "true"},
		data + "/models": {"skipped_excluded", "false"}}
	paths, got := field(rows, "path"), map[string]outcome{}
	for i, path := range paths {
		got[path] = outcome{field(rows, "status")[i], field(rows, "secret")[i]}
	}
	if len(paths) != len(want) || !reflect.DeepEqual(got, want) {
		t.Errorf("status and secret flag by path %q in %d rows, want %q: a directory is walked "+
			"in its own order, and only the volume's config is a credential", got, len(paths), want)
	}
	if collector.Excluded(home, home+"/models") || collector.IsSecret(home, "config") {
		t.Error("a volume line applies to a home through the home tests")
	}
}

func TestAVolumeLineLeavesAHomesLibraryWhileAHomeLineInAnotherCaseApplies(t *testing.T) {
	root := fixture{files: []string{"home/u/Library/notes", "home/u/tool/Cache/blob",
		"volumes/v/_data/library/x"}}.build(t)
	home, data := root+"/home/u", root+"/volumes/v/_data"
	collector, rows, _ := testCollector(t, root, nil)
	collector.MaxSize, collector.SecretGlobs = 0, nil
	collector.Excludes, collector.VolumeExcludes = []string{"*/cache"}, []string{"library"}
	ctx := context.Background()
	_, err := collector.CollectPath(ctx, "u", home, "agent", home)
	mustDo(t, err)
	_, err = collector.CollectVolume(ctx, "docker", "agent", data)
	mustDo(t, err)
	want := map[string]string{home + "/Library/notes": "collected",
		home + "/tool/Cache": "skipped_excluded", data + "/library": "skipped_excluded"}
	got := map[string]string{}
	for i, path := range field(rows, "path") {
		got[path] = field(rows, "status")[i]
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("statuses by path %q, want %q: a directory is walked in its own order", got, want)
	}
}
