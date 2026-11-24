package stage

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestRunOutputHolds(t *testing.T) {
	output := RunOutput{RunName: "host_t_agent-artifacts"}
	foldsCase := syspath.Equal("A", "a")
	cases := []struct {
		name   string
		output RunOutput
		path   string
		want   bool
	}{
		{"the staging directory", output, "/img/h/.claude/m/.stage-host_t_agent-artifacts", true},
		{"a path below it", output, "/img/h/.claude/m/.stage-host_t_agent-artifacts/fs/x", true},
		{"the log", output, "/img/h/.claude/m/host_t_agent-artifacts.log", true},
		{"the archive being written", output,
			"/img/h/.claude/m/host_t_agent-artifacts.tar.gz.partial", true},
		{"the log in any directory, however it is reached", output,
			"/alias/h/.CLAUDE/x/host_t_agent-artifacts.log", true},
		{"a copy of the log below any directory", output,
			"/img/h/d/.stage-host_t_agent-artifacts/fs/img/h/host_t_agent-artifacts.log", true},
		{"the staging directory in another letter case, held where the platform ignores case",
			output, "/img/.STAGE-HOST_T_agent-artifacts", foldsCase},
		{"another run's log", output, "/img/h/.claude/m/host_u_agent-artifacts.log", false},
		{"a name that only starts with the run's name", output,
			"/img/h/.claude/m/host_t_agent-artifactsx", false},
		{"a name that holds the run's name after other bytes", output,
			"/img/h/x.host_t_agent-artifacts.log", false},
		{"the run's name alone", output, "/img/h/host_t_agent-artifacts", false},
		{"another staging directory", output, "/img/.stage-host_u_agent-artifacts", false},
		{"a path with no output", output, "/img/h/.claude/m/notes", false},
		{"the zero RunOutput holds nothing", RunOutput{}, "/.stage-/.", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.output.Holds(testCase.path); got != testCase.want {
				t.Errorf("Holds(%s) = %v, want %v", testCase.path, got, testCase.want)
			}
		})
	}
}
