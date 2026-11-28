package parserkit

import "testing"

func TestEvidenceIDsGiveTheEvidencesSpellingOfADiskName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		ids       []string
		lookup    string
		want      string
		wantFound bool
	}{
		{"the name itself when the set holds it", []string{"ab1"}, "ab1", "ab1", true},
		{"the one id equal to it ignoring ASCII case", []string{"ab1", "cd2"}, "AB1", "ab1", true},
		{"none when two ids are equal to it ignoring case and neither is exact", []string{"AB1", "ab1"}, "Ab1", "", false},
		{"the exact id among two equal ignoring case", []string{"AB1", "ab1"}, "ab1", "ab1", true},
		{"one id added twice is no clash", []string{"ab1", "ab1"}, "AB1", "ab1", true},
		{"none when no id matches", []string{"ab1"}, "ab2", "", false},
		{"none in an empty set", nil, "ab1", "", false},
		{"only ASCII letters fold", []string{"é1"}, "É1", "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var ids EvidenceIDs
			for _, each := range test.ids {
				ids.Add(each)
			}
			got, found := ids.Spelling(test.lookup)
			if got != test.want || found != test.wantFound {
				t.Errorf("Spelling(%q) = %q, %v; want %q, %v", test.lookup, got, found, test.want, test.wantFound)
			}
		})
	}
}
