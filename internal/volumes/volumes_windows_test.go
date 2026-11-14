package volumes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

func TestUnmatchedVolumeRowCarriesTheDataDirectoryAttributes(t *testing.T) {
	fixture := newSetup(t, false)
	fixture.write(t, "var/lib/docker/volumes/postgres/_data/PG_VERSION", "16")
	var notes []string
	if _, err := Collect(context.Background(), fixture.collector, fixture.bases(), nil, &notes); err != nil {
		t.Fatal(err)
	}
	meta, err := fsmeta.Lstat(fixture.root + "/var/lib/docker/volumes/postgres/_data")
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(fixture.rows.Bytes(), &row); err != nil {
		t.Fatalf("row %s: %v", fixture.rows, err)
	}
	want := fsmeta.AttributesString(meta.Attributes)
	if row["attributes"] != want || row["owner"] != "" || row["mode"] != "" {
		t.Errorf("row %s, want attributes %q, owner and mode empty", fixture.rows, want)
	}
}
