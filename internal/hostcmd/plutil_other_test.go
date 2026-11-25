//go:build !darwin

package hostcmd

import "testing"

func TestPlutilIsOffForThisPlatform(t *testing.T) {
	if NewPlutil(&Fake{}).enabled {
		t.Error("plutil is on where it is not macOS")
	}
}
