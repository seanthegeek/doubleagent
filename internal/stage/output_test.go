package stage

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestRunOutputHolds(t *testing.T) {
	output := RunOutput{OutDir: "/img/h/.claude/m", RunName: "host_t_agent-artifacts"}
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
		{"another run's log", output, "/img/h/.claude/m/host_u_agent-artifacts.log", false},
		{"a name that only starts with the run's name", output,
			"/img/h/.claude/m/host_t_agent-artifactsx", false},
		{"the output directory itself", output, "/img/h/.claude/m", false},
		{"another file in the output directory", output, "/img/h/.claude/m/notes", false},
		{"the run's name in another directory", output,
			"/img/h/.claude/host_t_agent-artifacts.log", false},
		{"the run's name below another directory of the output directory", output,
			"/img/h/.claude/m/d/host_t_agent-artifacts.log", false},
		{"another spelling of the output directory, held where the platform ignores case",
			output, "/img/h/.CLAUDE/m/host_t_agent-artifacts.log", foldsCase},
		{"an output directory with a trailing slash", RunOutput{OutDir: "/", RunName: "r"},
			"/r.log", true},
		{"the zero RunOutput holds nothing", RunOutput{}, "/.stage-", false},
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
