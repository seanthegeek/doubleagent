package relocation

import (
	"slices"
	"testing"
)

func TestSystemRegistryStrings(t *testing.T) {
	cases := []struct {
		name, subkey string
		wantPath     bool
	}{
		{"the system environment key lists Path and reads without a reason",
			systemEnvironmentKey, true},
		{"a missing key gives no names and no reason",
			`SOFTWARE\doubleagent-test-no-such-key\Environment`, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := SystemRegistry().Strings(LocalMachine, test.subkey)
			if got.Reason != "" {
				t.Errorf("reason = %q, want none", got.Reason)
			}
			if hasPath := slices.Contains(got.Names, "Path"); hasPath != test.wantPath {
				t.Errorf("Path among the names = %v, want %v; names %q", hasPath, test.wantPath,
					got.Names)
			}
			if !test.wantPath && (len(got.Names) != 0 || len(got.Text) != 0) {
				t.Errorf("a missing key gave names %q and text %q", got.Names, got.Text)
			}
		})
	}
}

func TestSystemRegistryReadsTheTypeOfEachValue(t *testing.T) {
	got := SystemRegistry().Strings(LocalMachine, systemEnvironmentKey)
	// Path is REG_EXPAND_SZ on every Windows install, and the standard
	// NUMBER_OF_PROCESSORS and OS values are REG_SZ.
	if !got.Expandable["Path"] || got.Text["Path"] == "" {
		t.Errorf("Path: expandable %v, text %q; want an expandable, non-empty value",
			got.Expandable["Path"], got.Text["Path"])
	}
	if text, ok := got.Text["OS"]; !ok || got.Expandable["OS"] || text != "Windows_NT" {
		t.Errorf("OS = %q (present %v, expandable %v), want the REG_SZ Windows_NT", text, ok,
			got.Expandable["OS"])
	}
}

func TestSystemRegistryListsLoadedHives(t *testing.T) {
	// HKEY_USERS always holds the hive of the local system account, S-1-5-18.
	if hives := SystemRegistry().LoadedHives(); !slices.Contains(hives, "S-1-5-18") {
		t.Errorf("loaded hives = %q, want S-1-5-18 among them", hives)
	}
}
