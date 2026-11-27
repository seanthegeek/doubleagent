package fsmeta

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// recordedOpen is a sourceOpen whose steps record what they are called
// with; createErrs fails the opens in turn, and refuse makes admit refuse.
func recordedOpen(calls *[]string, createErrs []error, refuse bool) sourceOpen {
	opened := 0
	return sourceOpen{
		create: func(access uint32) (uintptr, error) {
			*calls = append(*calls, fmt.Sprintf("create %d", access))
			opened++
			if opened <= len(createErrs) && createErrs[opened-1] != nil {
				return 0, createErrs[opened-1]
			}
			return uintptr(opened), nil
		},
		admit: func(handle uintptr) error {
			*calls = append(*calls, fmt.Sprintf("admit %d", handle))
			if refuse {
				return errors.New("outside")
			}
			return nil
		},
		release:   func(handle uintptr) { *calls = append(*calls, fmt.Sprintf("release %d", handle)) },
		holdRight: func(access uint32) bool { return access == 2 },
		hold: func(handle uintptr) string {
			*calls = append(*calls, fmt.Sprintf("hold %d", handle))
			return ""
		},
		report: func(handle uintptr, reason string) {
			*calls = append(*calls, fmt.Sprintf("report %d %q", handle, reason))
		},
	}
}

func TestAnEvidenceOpenAdmitsTheHandleBeforeAnythingElse(t *testing.T) {
	denied := errors.New("access denied")
	cases := []struct {
		name       string
		createErrs []error
		refuse     bool
		want       []string
		wantErr    bool
	}{
		{"an admitted handle with the right is held after admission", nil, false,
			[]string{"create 2", "admit 1", "hold 1", `report 1 ""`}, false},
		{"a refused handle with the right is released and never held or reported", nil, true,
			[]string{"create 2", "admit 1", "release 1"}, true},
		{"a read-only fallback is admitted before its refusal of the right is reported",
			[]error{denied}, false,
			[]string{"create 2", "create 1", "admit 2", `report 2 "right to hold refused"`}, false},
		{"a refused read-only fallback is released and never reported", []error{denied}, true,
			[]string{"create 2", "create 1", "admit 2", "release 2"}, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls []string
			_, err := recordedOpen(&calls, testCase.createErrs, testCase.refuse).run([]uint32{2, 1})
			if (err != nil) != testCase.wantErr || !reflect.DeepEqual(calls, testCase.want) {
				t.Errorf("calls %q, error %v; want %q", calls, err, testCase.want)
			}
		})
	}
	var calls []string
	if _, err := recordedOpen(&calls, nil, true).first(1); err == nil ||
		!reflect.DeepEqual(calls, []string{"create 1", "admit 1", "release 1"}) {
		t.Errorf("first: calls %q, error %v", calls, err)
	}
}
