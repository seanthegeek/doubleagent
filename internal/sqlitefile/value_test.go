package sqlitefile

import "testing"

func TestUTF8Text(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value Value
		want  string
	}{
		{"NULL is empty", Value{}, ""},
		{"an integer is in decimal", IntValue(-42), "-42"},
		{"a float is in its shortest form", FloatValue(2.5), "2.5"},
		{"text keeps its bytes, invalid ones included", TextValue("a\xffb"), "a\xffb"},
		{"a blob is decoded as UTF-8", BlobValue([]byte(`{"a":1}`)), `{"a":1}`},
		{"a blob's invalid bytes each become U+FFFD", BlobValue([]byte("a\xff\xfeb")), "a\uFFFD\uFFFDb"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.value.UTF8Text(); got != test.want {
				t.Errorf("UTF8Text() = %q, want %q", got, test.want)
			}
		})
	}
}
