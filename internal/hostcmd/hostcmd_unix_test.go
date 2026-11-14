//go:build unix

package hostcmd

import "testing"

// This runs /bin/sh, which Windows does not have.

func TestSystemRunsProgramsInTheCLocale(t *testing.T) {
	cases := []struct {
		name string
		run  func(Runner) (string, error)
		want string
	}{
		{"sh sees LC_ALL=C", func(runner Runner) (string, error) {
			output, err := runner.Output("/bin/sh", "-c", "printf %s \"$LC_ALL\"")
			return string(output), err
		}, "C"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.run(System())
			if err != nil || got != testCase.want {
				t.Fatalf("got %q, %v; want %q", got, err, testCase.want)
			}
		})
	}
}
