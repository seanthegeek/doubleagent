package analyze

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// exampleStart and exampleEnd open the comments that mark an example block.
const (
	exampleStart = "<!-- example output: "
	exampleEnd   = "<!-- end example output: "
)

// exampleBlock is one marked block of the page.
type exampleBlock struct {
	// command is the line of the sh block above the start marker.
	command string
	// body is the text inside the block's fence, each line ended by LF.
	body string
	// bodyStart and bodyEnd are the byte offsets of body in the page.
	bodyStart, bodyEnd int
}

// findExampleBlock returns the block of the page marked name: the text
// between the fence lines that follow its start marker and precede its end
// marker, and the one line of the sh block that ends just above the start
// marker, blank lines between them allowed.
func findExampleBlock(page, name string) (exampleBlock, error) {
	start := strings.Index(page, exampleStart+name+" -->\n")
	end := strings.Index(page, exampleEnd+name+" -->")
	if start < 0 || end < start || strings.Count(page, exampleStart+name+" -->") != 1 {
		return exampleBlock{}, errors.New("the page has no single block marked " + name)
	}
	opening := start + len(exampleStart+name+" -->\n")
	fence, _, _ := strings.Cut(page[opening:], "\n")
	closing := strings.LastIndex(page[:end], "```\n")
	if !strings.HasPrefix(fence, "```") || closing < opening+len(fence) || page[closing:end] != "```\n" {
		return exampleBlock{}, errors.New("block " + name + " is not one fenced block between its markers")
	}
	command, err := commandAbove(page[:start])
	if err != nil {
		return exampleBlock{}, fmt.Errorf("block %s: %w", name, err)
	}
	bodyStart := opening + len(fence) + 1
	return exampleBlock{command: command, body: page[bodyStart:closing], bodyStart: bodyStart,
		bodyEnd: closing}, nil
}

// commandAbove returns the one line of the sh block that ends the text
// before, after any blank lines.
func commandAbove(before string) (string, error) {
	lines := strings.Split(strings.TrimRight(before, "\n"), "\n")
	if len(lines) < 3 || lines[len(lines)-1] != "```" || lines[len(lines)-3] != "```sh" {
		return "", errors.New("no one-line sh block shows the command above the block")
	}
	return lines[len(lines)-2], nil
}

// writeExamplePage rewrites the body of each block named in outputs and
// writes the page, leaving every other byte as it was.
func writeExamplePage(t *testing.T, page string, outputs map[string]string) {
	t.Helper()
	for _, test := range exampleCases() {
		output, found := outputs[test.name]
		if !found {
			continue
		}
		block, err := findExampleBlock(page, test.name)
		if err != nil {
			t.Fatal(err)
		}
		if output != "" && !strings.HasSuffix(output, "\n") {
			output += "\n"
		}
		page = page[:block.bodyStart] + output + page[block.bodyEnd:]
	}
	if err := osfile.WriteFile(examplePage, []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("rewrote the example blocks of %s", examplePage)
}

// lineDiff returns the lines of want missing from got, marked "-", and the
// lines of got missing from want, marked "+", each with its line number, in
// the order of a longest common run of lines; at most forty are listed.
func lineDiff(want, got string) string {
	left, right := strings.Split(want, "\n"), strings.Split(got, "\n")
	common := make([][]int, len(left)+1)
	for i := range common {
		common[i] = make([]int, len(right)+1)
	}
	for i := len(left) - 1; i >= 0; i-- {
		for j := len(right) - 1; j >= 0; j-- {
			if left[i] == right[j] {
				common[i][j] = common[i+1][j+1] + 1
			} else {
				common[i][j] = max(common[i+1][j], common[i][j+1])
			}
		}
	}
	var lines []string
	i, j := 0, 0
	for (i < len(left) || j < len(right)) && len(lines) < 40 {
		switch {
		case i < len(left) && j < len(right) && left[i] == right[j]:
			i, j = i+1, j+1
		case i < len(left) && (j == len(right) || common[i+1][j] >= common[i][j+1]):
			lines = append(lines, fmt.Sprintf("-%d: %s", i+1, left[i]))
			i++
		default:
			lines = append(lines, fmt.Sprintf("+%d: %s", j+1, right[j]))
			j++
		}
	}
	return strings.Join(lines, "\n")
}

func TestLineDiffListsTheLinesEachSideLacks(t *testing.T) {
	cases := []struct {
		name, want, got, diff string
	}{
		{"equal texts give no lines", "a\nb\n", "a\nb\n", ""},
		{"a changed line is removed then added", "a\nb\nc", "a\nx\nc", "-2: b\n+2: x"},
		{"an added line is listed alone", "a\nc", "a\nb\nc", "+2: b"},
		{"a removed line is listed alone", "a\nb\nc", "a\nc", "-2: b"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := lineDiff(test.want, test.got); got != test.diff {
				t.Errorf("lineDiff = %q, want %q", got, test.diff)
			}
		})
	}
}

func TestSplitCommandLineReadsTheFormsThePageUses(t *testing.T) {
	cases := []struct {
		name, line string
		words      []string
		fails      bool
	}{
		{"plain words split at spaces", "doubleagent detect /in", []string{"doubleagent", "detect", "/in"}, false},
		{"a quoted word keeps its spaces and backslashes", `x --match 'a\.b|c d' y`,
			[]string{"x", "--match", `a\.b|c d`, "y"}, false},
		{"an unclosed quote fails", "x 'a", nil, true},
		{"a quote inside a word fails", "x a'b'", nil, true},
		{"a dollar sign fails", "x $HOME", nil, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			words, err := splitCommandLine(test.line)
			if (err != nil) != test.fails || strings.Join(words, "\x00") != strings.Join(test.words, "\x00") {
				t.Errorf("splitCommandLine(%q) = %q, %v", test.line, words, err)
			}
		})
	}
}
