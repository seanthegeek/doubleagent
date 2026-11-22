package walk

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// relativeLink is a symbolic link whose target is the relative name given.
func relativeLink(name string) fsmeta.Reparse {
	return fsmeta.Reparse{Tag: fsmeta.TagSymlink, Substitute: name, Relative: true}
}

// absoluteLink is a symbolic link whose target is the absolute name given.
func absoluteLink(name string) fsmeta.Reparse {
	return fsmeta.Reparse{Tag: fsmeta.TagSymlink, Substitute: name}
}

// junctionLink is a junction whose target is the name given.
func junctionLink(name string) fsmeta.Reparse {
	return fsmeta.Reparse{Tag: fsmeta.TagMountPoint, Substitute: name}
}

// An acceptedLink is a link's data and the form it is followed as.
type acceptedLink struct {
	name string
	data fsmeta.Reparse
	want linkForm
}

// acceptedLinks returns the link forms that are followed.
func acceptedLinks() []acceptedLink {
	return []acceptedLink{
		{"a relative name", relativeLink(`a\b`), linkForm{"a/b", relativeTarget}},
		{"a relative name climbing", relativeLink(`..\..\x`), linkForm{"../../x", relativeTarget}},
		{"a relative name with a dot", relativeLink(`.\x`), linkForm{"./x", relativeTarget}},
		{"a relative name with a trailing separator", relativeLink(`a\b\`), linkForm{"a/b", relativeTarget}},
		{"the link's drive root", relativeLink(`\`), linkForm{"/", rootedTarget}},
		{"a name rooted on the link's drive", relativeLink(`\x\y`), linkForm{"/x/y", rootedTarget}},
		{"a drive path", absoluteLink(`\??\C:\x\y`), linkForm{"C:/x/y", absoluteTarget}},
		{"a drive root", absoluteLink(`\??\d:\`), linkForm{"d:", absoluteTarget}},
		{"a drive path climbing", absoluteLink(`\??\C:\x\..\y`), linkForm{"C:/x/../y", absoluteTarget}},
		{"a share path", absoluteLink(`\??\UNC\srv\share\x`), linkForm{"//srv/share/x", absoluteTarget}},
		{"a junction to a drive path", junctionLink(`\??\C:\img\x`), linkForm{"C:/img/x", absoluteTarget}},
		{"a junction to a drive root", junctionLink(`\??\E:\`), linkForm{"E:", absoluteTarget}},
	}
}

// A refusedLink is a link's data that is not followed.
type refusedLink struct {
	name string
	data fsmeta.Reparse
}

// refusedLinks returns the link forms that are refused.
func refusedLinks() []refusedLink {
	return []refusedLink{
		{"a volume GUID name", absoluteLink(`\??\Volume{3c1e8b9a-0000-0000-0000-100000000000}\x`)},
		{"a volume GUID name as Readlink prints it", absoluteLink(`\\?\Volume{3c1e8b9a}\x`)},
		{"GLOBALROOT", absoluteLink(`\??\GLOBALROOT\Device\HarddiskVolumeShadowCopy1\`)},
		{"a device path", absoluteLink(`\Device\HarddiskVolume3\x`)},
		{"the global namespace", absoluteLink(`\GLOBAL??\C:\x`)},
		{"a device namespace path", absoluteLink(`\\.\C:\x`)},
		{"a long path", absoluteLink(`\\?\C:\x`)},
		{"a drive path without its prefix", absoluteLink(`C:\x`)},
		{"a drive-relative name", absoluteLink(`\??\C:x`)},
		{"a drive head alone", absoluteLink(`\??\C:`)},
		{"a share with no share name", absoluteLink(`\??\UNC\srv`)},
		{"a share with an empty server", absoluteLink(`\??\UNC\\share\x`)},
		{"an empty absolute name", absoluteLink(``)},
		{"a relative-marked drive-relative name", relativeLink(`C:x`)},
		{"a relative-marked drive path", relativeLink(`C:\x`)},
		{"a relative-marked NT name", relativeLink(`\??\C:\x`)},
		{"a relative-marked device path", relativeLink(`\\.\C:\x`)},
		{"a relative-marked share", relativeLink(`\\srv\share\x`)},
		{"a relative-marked volume GUID", relativeLink(`\\?\Volume{1}\x`)},
		{"an empty relative name", relativeLink(``)},
		{"a junction to a share", junctionLink(`\??\UNC\srv\share\x`)},
		{"a junction to a volume GUID", junctionLink(`\??\Volume{1}\`)},
		{"a junction climbing", junctionLink(`\??\C:\img\..\x`)},
		{"a junction with a dot", junctionLink(`\??\C:\img\.\x`)},
		{"a junction without the prefix", junctionLink(`C:\x`)},
		{"an empty junction", junctionLink(``)},
		{"another tag", fsmeta.Reparse{Tag: 0x8000001b, Substitute: `\??\C:\x`}},
		{"no tag", fsmeta.Reparse{Substitute: `a`, Relative: true}},
	}
}

func TestOnlyKnownLinkFormsAreFollowed(t *testing.T) {
	for _, testCase := range acceptedLinks() {
		t.Run(testCase.name, func(t *testing.T) {
			got, ok := acceptLink(testCase.data)
			if !ok || got != testCase.want {
				t.Errorf("acceptLink = %+v, %v; want %+v, true", got, ok, testCase.want)
			}
		})
	}
	for _, testCase := range refusedLinks() {
		t.Run(testCase.name, func(t *testing.T) {
			if got, ok := acceptLink(testCase.data); ok {
				t.Errorf("acceptLink = %+v, true; want a refusal", got)
			}
		})
	}
}
