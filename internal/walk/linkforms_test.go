package walk

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

func TestOnlyKnownLinkFormsAreFollowed(t *testing.T) {
	relative := func(name string) fsmeta.Reparse {
		return fsmeta.Reparse{Tag: fsmeta.TagSymlink, Substitute: name, Relative: true}
	}
	absolute := func(name string) fsmeta.Reparse {
		return fsmeta.Reparse{Tag: fsmeta.TagSymlink, Substitute: name}
	}
	junction := func(name string) fsmeta.Reparse {
		return fsmeta.Reparse{Tag: fsmeta.TagMountPoint, Substitute: name}
	}
	accepted := []struct {
		name string
		data fsmeta.Reparse
		want linkForm
	}{
		{"a relative name", relative(`a\b`), linkForm{"a/b", relativeTarget}},
		{"a relative name climbing", relative(`..\..\x`), linkForm{"../../x", relativeTarget}},
		{"a relative name with a dot", relative(`.\x`), linkForm{"./x", relativeTarget}},
		{"a relative name with a trailing separator", relative(`a\b\`), linkForm{"a/b", relativeTarget}},
		{"the link's drive root", relative(`\`), linkForm{"/", rootedTarget}},
		{"a name rooted on the link's drive", relative(`\x\y`), linkForm{"/x/y", rootedTarget}},
		{"a drive path", absolute(`\??\C:\x\y`), linkForm{"C:/x/y", absoluteTarget}},
		{"a drive root", absolute(`\??\d:\`), linkForm{"d:", absoluteTarget}},
		{"a drive path climbing", absolute(`\??\C:\x\..\y`), linkForm{"C:/x/../y", absoluteTarget}},
		{"a share path", absolute(`\??\UNC\srv\share\x`), linkForm{"//srv/share/x", absoluteTarget}},
		{"a junction to a drive path", junction(`\??\C:\img\x`), linkForm{"C:/img/x", absoluteTarget}},
		{"a junction to a drive root", junction(`\??\E:\`), linkForm{"E:", absoluteTarget}},
	}
	for _, testCase := range accepted {
		t.Run(testCase.name, func(t *testing.T) {
			got, ok := acceptLink(testCase.data)
			if !ok || got != testCase.want {
				t.Errorf("acceptLink = %+v, %v; want %+v, true", got, ok, testCase.want)
			}
		})
	}
	refused := []struct {
		name string
		data fsmeta.Reparse
	}{
		{"a volume GUID name", absolute(`\??\Volume{3c1e8b9a-0000-0000-0000-100000000000}\x`)},
		{"a volume GUID name as Readlink prints it", absolute(`\\?\Volume{3c1e8b9a}\x`)},
		{"GLOBALROOT", absolute(`\??\GLOBALROOT\Device\HarddiskVolumeShadowCopy1\`)},
		{"a device path", absolute(`\Device\HarddiskVolume3\x`)},
		{"the global namespace", absolute(`\GLOBAL??\C:\x`)},
		{"a device namespace path", absolute(`\\.\C:\x`)},
		{"a long path", absolute(`\\?\C:\x`)},
		{"a drive path without its prefix", absolute(`C:\x`)},
		{"a drive-relative name", absolute(`\??\C:x`)},
		{"a drive head alone", absolute(`\??\C:`)},
		{"a share with no share name", absolute(`\??\UNC\srv`)},
		{"a share with an empty server", absolute(`\??\UNC\\share\x`)},
		{"an empty absolute name", absolute(``)},
		{"a relative-marked drive-relative name", relative(`C:x`)},
		{"a relative-marked drive path", relative(`C:\x`)},
		{"a relative-marked NT name", relative(`\??\C:\x`)},
		{"a relative-marked device path", relative(`\\.\C:\x`)},
		{"a relative-marked share", relative(`\\srv\share\x`)},
		{"a relative-marked volume GUID", relative(`\\?\Volume{1}\x`)},
		{"an empty relative name", relative(``)},
		{"a junction to a share", junction(`\??\UNC\srv\share\x`)},
		{"a junction to a volume GUID", junction(`\??\Volume{1}\`)},
		{"a junction climbing", junction(`\??\C:\img\..\x`)},
		{"a junction with a dot", junction(`\??\C:\img\.\x`)},
		{"a junction without the prefix", junction(`C:\x`)},
		{"an empty junction", junction(``)},
		{"another tag", fsmeta.Reparse{Tag: 0x8000001b, Substitute: `\??\C:\x`}},
		{"no tag", fsmeta.Reparse{Substitute: `a`, Relative: true}},
	}
	for _, testCase := range refused {
		t.Run(testCase.name, func(t *testing.T) {
			if got, ok := acceptLink(testCase.data); ok {
				t.Errorf("acceptLink = %+v, true; want a refusal", got)
			}
		})
	}
}
