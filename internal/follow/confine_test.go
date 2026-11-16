package follow

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestConfinementAdmitsOnlyAHandleUnderTheRoot(t *testing.T) {
	const root = `\Device\HarddiskVolume3\cases\img`
	cases := []struct {
		name  string
		place string
		err   error
		admit bool
	}{
		{"the root itself", root, nil, true},
		{"a file under the root", root + `\Users\alice\notes`, nil, true},
		{"a sibling sharing the root's prefix", root + `2\Users\alice\notes`, nil, false},
		{"another case of the root, which a case-sensitive directory keeps apart",
			`\Device\HarddiskVolume3\cases\IMG\x`, nil, false},
		{"another volume", `\Device\HarddiskVolume4\cases\img\x`, nil, false},
		{"the analyst's own files", `\Device\HarddiskVolume3\Users\analyst\notes`, nil, false},
		{"a place that cannot be asked", "", errors.New("access denied"), false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var reported []string
			confinement := newConfinement(root,
				func(*os.File) (string, error) { return testCase.place, testCase.err },
				func(path string) { reported = append(reported, path) })
			err := confinement.Admit("C:/cases/img/x", nil)
			if testCase.admit {
				if err != nil || len(reported) != 0 {
					t.Errorf("Admit = %v, reported %q; want it admitted", err, reported)
				}
				return
			}
			want := Refusal{Link: "C:/cases/img/x", Kind: OpenedOutside}
			if refusal, refused := OutOfRoot(err); !refused || refusal != want {
				t.Errorf("Admit = %v; want the refusal %+v", err, want)
			}
			if !reflect.DeepEqual(reported, []string{"C:/cases/img/x"}) {
				t.Errorf("reported %q", reported)
			}
		})
	}
}

func TestConfinementReportsEachPathOnce(t *testing.T) {
	var reported []string
	confinement := newConfinement(`\Device\Mup\server\share\img\`,
		func(*os.File) (string, error) { return `\Device\Mup\server\other\x`, nil },
		func(path string) { reported = append(reported, path) })
	for _, path := range []string{"a", "b", "a"} {
		if err := confinement.Admit(path, nil); err == nil {
			t.Fatalf("Admit(%s) admitted a handle outside the root", path)
		}
	}
	if !reflect.DeepEqual(reported, []string{"a", "b"}) {
		t.Errorf("reported %q, want a and b once each", reported)
	}
	// A root given with its trailing separator (a volume's own root) still
	// admits what is under it.
	inside := newConfinement(`\Device\HarddiskVolume7\`,
		func(*os.File) (string, error) { return `\Device\HarddiskVolume7\x`, nil }, nil)
	if err := inside.Admit("x", nil); err != nil {
		t.Errorf("Admit under a volume root: %v", err)
	}
}

func TestANilConfinementAdmitsEverything(t *testing.T) {
	var confinement *Confinement
	if err := confinement.Admit("x", nil); err != nil {
		t.Errorf("Admit = %v", err)
	}
}

func TestTheOpenedOutsideRefusalIsWorded(t *testing.T) {
	got := "C:\\img\\x" + Reason(Refusal{Kind: OpenedOutside}, "")
	want := `C:\img\x: the root check allowed it but Windows opened it outside the image root;` +
		" nothing from it was used"
	if got != want {
		t.Errorf("%q, want %q", got, want)
	}
}
