//go:build parity

package relocation

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// collectorScript returns the text of the sh collector.
func collectorScript(t *testing.T) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("..", "..", "collectors", "collect-agent-artifacts.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

// between returns the text of script after the line holding start and
// before the next line equal to end.
func between(t *testing.T, script, start, end string) string {
	t.Helper()
	_, after, found := strings.Cut(script, start)
	if !found {
		t.Fatalf("no %q in the collector", start)
	}
	_, after, _ = strings.Cut(after, "\n")
	body, _, found := strings.Cut(after, "\n"+end+"\n")
	if !found {
		t.Fatalf("no %q after %q in the collector", end, start)
	}
	return body
}

// runShell runs script under dash with args, in the C locale and with the
// system tools first on the search path, and returns its standard output.
func runShell(t *testing.T, input []byte, script string, args ...string) string {
	t.Helper()
	command := exec.Command("dash", append([]string{"-c", script, "sh"}, args...)...)
	command.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin", "LC_ALL=C"}
	command.Stdin = bytes.NewReader(input)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("dash: %v", err)
	}
	return string(output)
}

// formatAssignments writes assignments as the awk readers print them.
func formatAssignments(assignments []Assignment) string {
	var text strings.Builder
	for _, assignment := range assignments {
		outcome := "=" + assignment.Value
		if assignment.Reason != "" {
			outcome = "!" + assignment.Reason
		}
		fmt.Fprintf(&text, "%d|%s|%s\n", assignment.Line, assignment.Name, outcome)
	}
	return text.String()
}

// readerInputs returns the smoke test's files and generated ones built from
// the tokens the readers treat specially.
func readerInputs() []string {
	inputs := []string{smokeEnvironment(), smokeValuesConf(),
		"CODEX_HOME=/opt/cr\\\r\nlf/\r\n", "CODEX_HOME=\"/opt/x\\\"",
		"CODEX_HOME=\"/opt/a\nCLAUDE_CONFIG_DIR=/opt/forged\n\"\n" +
			"HERMES_HOME=\"/opt/x\\\"\nAMP_HOME=/opt/h\n",
		"CODEX_HOME=/a\rCLAUDE_CONFIG_DIR=/c\n", "HERMES_HOME=/n\x00ul\nCODEX_HOME=/opt/afternul\n",
		"CODEX_HOME=$HOME/x\nCODEX_HOME=${HOME}/y\nCODEX_HOME=${HOME:+z}\n"}
	tokens := []string{"CODEX_HOME", "HERMES_HOME", "=", "/opt", "/", `\`, `"`, "'", "$", "$$",
		"HOME", "{", "}", ":-", " ", "\t", "\r", "#", ";", "\n", "\x00", "\x01", "x", "|"}
	random := rand.New(rand.NewSource(7))
	for range 400 {
		var text strings.Builder
		for range 4 + random.Intn(20) {
			text.WriteString(tokens[random.Intn(len(tokens))])
		}
		inputs = append(inputs, "CODEX_HOME="+text.String())
	}
	return inputs
}

func TestValueReadersAgreeWithTheShAwk(t *testing.T) {
	script := collectorScript(t)
	programs := map[string]string{}
	for kind, label := range map[string]string{"environment": "EOF_ENVIRONMENT_VALUES_AWK",
		"environment.d": "EOF_ENVIRONMENT_D_VALUES_AWK"} {
		programs[kind] = between(t, script, "<<'"+label+"'", label)
	}
	names := testNames()
	pipeline := `tr '\000\001' '\001\002' | RESOLVE_HOME=$2 awk -v names="$3" "$1"`
	for i, input := range readerInputs() {
		for kind, program := range programs {
			want := runShell(t, []byte(input), pipeline, program, "/home/alice", strings.Join(names, ","))
			var got []Assignment
			var err error
			if kind == "environment" {
				got, err = ResolveEnvironment(strings.NewReader(input), names)
			} else {
				got, err = ResolveEnvironmentD(strings.NewReader(input), names, "/home/alice")
			}
			if err != nil || formatAssignments(got) != want {
				t.Errorf("%s input %d %q:\ngot  %q\nwant %q", kind, i, input, formatAssignments(got), want)
			}
		}
	}
}

func TestBoundedNamesAgreeWithTheShGrepAndAwk(t *testing.T) {
	script := collectorScript(t)
	_, program, _ := strings.Cut(script, `-v fold="$_lbn_fold" '`)
	program, _, _ = strings.Cut(program, "'\n")
	names := []string{"CODEX_HOME", "ENV", "CLAUDE_CONFIG_DIR"}
	options := "-e CODEX_HOME -e ENV -e CLAUDE_CONFIG_DIR"
	pipeline := `grep $4 -a -n -F ` + options +
		` | tr '\000' ' ' | awk -v names="$2" -v fold="$3" "$1"`
	tokens := []string{"CODEX_HOME", "codex_home", "ENV", "env", "XENV", "ENVX", ":", "$", "_", " ",
		"\x00", "\r", "\n", "#!/usr/bin/env", "é", "CLAUDE_CONFIG_DIR"}
	random := rand.New(rand.NewSource(11))
	for i := range 400 {
		var text strings.Builder
		for range 3 + random.Intn(16) {
			text.WriteString(tokens[random.Intn(len(tokens))])
		}
		for _, powerShell := range []bool{false, true} {
			fold, caseOption := "0", "-e ENV"
			if powerShell {
				fold, caseOption = "1", "-i"
			}
			want := runShell(t, []byte(text.String()), pipeline, program, strings.Join(names, ","),
				fold, caseOption)
			hits, err := BoundedNames(strings.NewReader(text.String()), names, powerShell)
			var got strings.Builder
			for _, hit := range hits {
				got.WriteString(strconv.Itoa(hit.Line) + "|" + hit.Name + "\n")
			}
			if err != nil || got.String() != want {
				t.Errorf("input %d %q, PowerShell %v:\ngot  %q\nwant %q", i, text.String(),
					powerShell, got.String(), want)
			}
		}
	}
}

func TestAcceptValueAgreesWithTheShFunction(t *testing.T) {
	script := collectorScript(t)
	function := "accept_resolved_value() {\n" +
		between(t, script, "accept_resolved_value() {", "}") + "\n}\n"
	program := "NEWLINE='\n'\nCONTROL_CHARACTERS=$(printf '\\001\\002\\003\\004\\005\\006\\007\\010" +
		"\\011\\012\\013\\014\\015\\016\\017\\020\\021\\022\\023\\024\\025\\026\\027\\030\\031" +
		"\\032\\033\\034\\035\\036\\037\\177')\nROOT=$1; shift\n" + function +
		`for value in "$@"; do accept_resolved_value "$value"; ` +
		`printf '%s|%s\n' "$_ret_value" "$_ret_reason"; done`
	tokens := []string{"/", "//", "..", ".", "opt", "$", "{", "}", "1", "%", "A", "_", "`", `"`, "'",
		":", "file", "~", "\t", "\x1b", "\x7f", "é", "-", "+", "C"}
	random := rand.New(rand.NewSource(13))
	values := []string{"", "/", "//", ":x", "a:b", "C:\\x", "/opt/d/", "%A%", "%%", "$1"}
	for range 600 {
		var text strings.Builder
		for range 1 + random.Intn(8) {
			text.WriteString(tokens[random.Intn(len(tokens))])
		}
		values = append(values, text.String())
	}
	for _, root := range []string{"", "/mnt/image"} {
		output := runShell(t, nil, program, append([]string{root}, values...)...)
		lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
		for i, value := range values {
			accepted, reason := AcceptValue(value, root)
			if got := accepted + "|" + reason; i >= len(lines) || got != lines[i] {
				t.Errorf("AcceptValue(%q, %q) = %q, sh gives %q", value, root, got, lines[i])
			}
		}
	}
}
