package hostcmd

import (
	"strings"
	"testing"
)

// Fake is a Runner that answers from a table, for tests of the packages that
// take a Runner.
type Fake struct {
	Outputs map[string][]byte
	Calls   []string
}

// Output returns the canned output for NAME, or an error when there is none.
func (fake *Fake) Output(name string, args ...string) ([]byte, error) {
	fake.Calls = append(fake.Calls, strings.Join(append([]string{name}, args...), " "))
	output, ok := fake.Outputs[name]
	if !ok {
		return nil, &missingError{name}
	}
	return output, nil
}

// Available reports whether NAME has canned output.
func (fake *Fake) Available(name string) bool {
	_, ok := fake.Outputs[name]
	return ok
}

type missingError struct{ name string }

func (e *missingError) Error() string { return e.name + ": not available" }

func TestFakeIsARunner(t *testing.T) {
	var runner Runner = &Fake{Outputs: map[string][]byte{"getent": []byte("root:x:0:0\n")}}
	if !runner.Available("getent") || runner.Available("dscl") {
		t.Error("Available does not follow the table")
	}
	output, err := runner.Output("getent", "passwd")
	if err != nil || string(output) != "root:x:0:0\n" {
		t.Errorf("Output = %q, %v", output, err)
	}
	if _, err := runner.Output("dscl"); err == nil {
		t.Error("want an error for a program with no output")
	}
}

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

func TestSystemAvailable(t *testing.T) {
	runner := System()
	if !runner.Available("sh") {
		t.Error("sh should be available")
	}
	if runner.Available("no-such-program-doubleagent") {
		t.Error("a missing program should not be available")
	}
	if _, err := runner.Output("no-such-program-doubleagent"); err == nil {
		t.Error("running a missing program should fail")
	}
}
