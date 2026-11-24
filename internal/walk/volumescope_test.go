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
	paths := []string{home + "/config", home + "/models/m", data + "/config", data + "/models"}
	if got := field(rows, "path"); !reflect.DeepEqual(got, paths) {
		t.Fatalf("paths %q, want %q", got, paths)
	}
	statuses := []string{"collected", "collected", "collected", "skipped_excluded"}
	if got := field(rows, "status"); !reflect.DeepEqual(got, statuses) {
		t.Errorf("statuses %q, want %q", got, statuses)
	}
	if got := field(rows, "secret"); !reflect.DeepEqual(got, []string{"false", "false", "true", "false"}) {
		t.Errorf("secret flags %q: only the volume's config is a credential", got)
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
	paths := []string{home + "/Library/notes", home + "/tool/Cache", data + "/library"}
	if got := field(rows, "path"); !reflect.DeepEqual(got, paths) {
		t.Fatalf("paths %q, want %q", got, paths)
	}
	statuses := []string{"collected", "skipped_excluded", "skipped_excluded"}
	if got := field(rows, "status"); !reflect.DeepEqual(got, statuses) {
		t.Errorf("statuses %q, want %q", got, statuses)
	}
}
