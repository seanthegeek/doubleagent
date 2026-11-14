package manifest

import "testing"

func TestAppendWindowsFields(t *testing.T) {
	cases := []struct {
		name string
		row  Row
		want string
	}{
		{"both set", Row{Owner: `HOST\alice`, Attributes: "Archive, ReparsePoint"},
			`"owner":"HOST\\alice","attributes":"Archive, ReparsePoint",`},
		{"both empty", Row{}, `"owner":"","attributes":"",`},
		{"escaped", Row{Owner: "a\"b"}, `"owner":"a\"b","attributes":"",`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := string(appendWindowsFields(nil, testCase.row)); got != testCase.want {
				t.Errorf("fields = %s, want %s", got, testCase.want)
			}
		})
	}
}
