//go:build darwin

package hostcmd

import "testing"

func TestPlutilIsOnForThisPlatform(t *testing.T) {
	if !NewPlutil(&Fake{}).enabled {
		t.Error("plutil is off on macOS")
	}
}
