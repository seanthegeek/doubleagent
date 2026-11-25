//go:build unix

package users

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// plistHead and plistTail enclose the records of dscl's property list form.
const (
	plistHead = `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" ` +
		`"http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n" +
		`<plist version="1.0">` + "\n<array>\n"
	plistTail = "</array>\n</plist>\n"
)

// dsclRecord returns one record dictionary as dscl prints it: its home
// values, then its record names, each attribute left out when it has none.
func dsclRecord(homes, names []string) string {
	var record strings.Builder
	record.WriteString("\t<dict>\n")
	for _, attribute := range []struct {
		key    string
		values []string
	}{{dsclHomeKey, homes}, {dsclNameKey, names}} {
		if len(attribute.values) == 0 {
			continue
		}
		record.WriteString("\t\t<key>" + attribute.key + "</key>\n\t\t<array>\n")
		for _, value := range attribute.values {
			record.WriteString("\t\t\t<string>" + value + "</string>\n")
		}
		record.WriteString("\t\t</array>\n")
	}
	record.WriteString("\t</dict>\n")
	return record.String()
}

// dsclOutput returns dscl's property list of one record per user and home
// pair, a home of "" leaving the record without one.
func dsclOutput(pairs ...string) []byte {
	text := plistHead
	for i := 0; i+1 < len(pairs); i += 2 {
		var homes []string
		if pairs[i+1] != "" {
			homes = []string{pairs[i+1]}
		}
		text += dsclRecord(homes, []string{pairs[i]})
	}
	return []byte(text + plistTail)
}

// dsclCases returns the rules of dscl's property list form that
// dsclRecordHomes relies on, each as the text between the head and tail.
func dsclCases() []struct {
	name, records string
	want          []Home
} {
	return []struct {
		name, records string
		want          []Home
	}{
		{"a home holding a space is whole", dsclRecord([]string{"/opt/Sam Ple"}, []string{"sam"}),
			[]Home{{"sam", "/opt/Sam Ple"}}},
		{"each home of a record is a candidate under its name",
			dsclRecord([]string{"/srv/one", "/srv/two"}, []string{"sam"}),
			[]Home{{"sam", "/srv/one"}, {"sam", "/srv/two"}}},
		{"the first record name is the user", dsclRecord([]string{"/srv/s"}, []string{"sam", "alias"}),
			[]Home{{"sam", "/srv/s"}}},
		{"the name may come before the home", "<dict><key>" + dsclNameKey +
			"</key><array><string>sam</string></array><key>" + dsclHomeKey +
			"</key><array><string>/srv/s</string></array></dict>", []Home{{"sam", "/srv/s"}}},
		{"an entity is the character it names", dsclRecord([]string{"/srv/a&amp;b&#32;c"},
			[]string{"s&lt;m"}), []Home{{"s<m", "/srv/a&b c"}}},
		{"another key and its value are passed over", "<dict><key>other</key><dict><key>" +
			dsclHomeKey + "</key><array><string>/srv/no</string></array></dict><key>" +
			dsclHomeKey + "</key><array><integer>1</integer><string>/srv/s</string></array>" +
			"</dict>", []Home{{"", "/srv/s"}}},
		{"a home value that is no array gives no home", "<dict><key>" + dsclHomeKey +
			"</key><string>/srv/s</string></dict>", nil},
		{"an empty home gives no home", dsclRecord([]string{""}, []string{"sam"}), nil},
		{"a record with no home gives none", dsclRecord(nil, []string{"sam"}), nil},
		{"a record with no name gives its home under no name", dsclRecord([]string{"/srv/s"}, nil),
			[]Home{{"", "/srv/s"}}},
		{"empty segments are dropped", dsclRecord([]string{"//srv//s/"}, []string{"sam"}),
			[]Home{{"sam", "/srv/s"}}},
		{"a record whose keys and values do not alternate gives none",
			"<dict><array/><key>" + dsclHomeKey + "</key></dict>" +
				dsclRecord([]string{"/srv/s"}, []string{"sam"}), []Home{{"sam", "/srv/s"}}},
		{"a dictionary outside the top-level array is no record", "</array><dict><key>" +
			dsclHomeKey + "</key><array><string>/srv/no</string></array></dict><array>", nil},
	}
}

func TestDsclRecordHomes(t *testing.T) {
	for _, testCase := range dsclCases() {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := dsclRecordHomes(strings.NewReader(plistHead + testCase.records + plistTail))
			if err != nil || !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("got %q, %v; want %q", got, err, testCase.want)
			}
		})
	}
}

func TestDsclRecordHomesKeepsTheRecordsBeforeAFault(t *testing.T) {
	text := plistHead + dsclRecord([]string{"/srv/s"}, []string{"sam"}) + "<dict><key>"
	got, err := dsclRecordHomes(strings.NewReader(text))
	if err == nil || !reflect.DeepEqual(got, []Home{{"sam", "/srv/s"}}) {
		t.Errorf("got %q, %v; want sam's home and an error", got, err)
	}
}

func TestDsclHomesAsksForThePropertyListForm(t *testing.T) {
	var asked []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		asked = append([]string{name}, args...)
		return dsclOutput("sam", "/opt/Sam Ple"), nil
	}
	got := dsclHomes(context.Background(), run)
	want := []string{"dscl", "-plist", ".", "-readall", "/Users", "NFSHomeDirectory", "RecordName"}
	if !reflect.DeepEqual(asked, want) || !reflect.DeepEqual(got, []Home{{"sam", "/opt/Sam Ple"}}) {
		t.Errorf("ran %q, got %q", asked, got)
	}
}

// TestALiveRunFindsADsclHomeHoldingASpace runs live over a fixture root
// whose home parent lists the home under its directory's name: -u names the
// account, which only dscl gives.
func TestALiveRunFindsADsclHomeHoldingASpace(t *testing.T) {
	root := tree{dirs: []string{"Users/Sam Ple"}}.build(t)
	run := func(_ context.Context, name string, _ ...string) ([]byte, error) {
		if name == "dscl" {
			return dsclOutput("sam", root+"/Users/Sam Ple"), nil
		}
		return nil, errors.New(name + ": not installed")
	}
	report := Find(context.Background(), true, root, "sam", run, nil, nil, nil)
	if want := []Home{{"sam", root + "/Users/Sam Ple"}}; !reflect.DeepEqual(report.Homes, want) ||
		len(report.Unmatched) != 0 {
		t.Errorf("homes %q, unmatched %q; want %q and none", report.Homes, report.Unmatched, want)
	}
}
