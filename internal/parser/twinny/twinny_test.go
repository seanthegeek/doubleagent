package twinny

import (
	"slices"
	"strings"
	"testing"
)

func TestSplitThinkTakesOutReasoningBlocksClosedByTheirOwnTag(t *testing.T) {
	for _, test := range []struct {
		name, content, text string
		thoughts            []string
	}{
		{"no block", "plain", "plain", nil},
		{"a closed block", "<think>a</think>b", "b", []string{"a"}},
		{"the long tag", "<thinking>x</thinking> y", "y", []string{"x"}},
		{"an unclosed block runs to the end", "<think>open", "", []string{"open"}},
		{"an unclosed block stops before a final LF", "<think>open\n", "", []string{"open"}},
		{"another tag's close does not end a block", "<think>a</thinking>b", "", []string{"a</thinking>b"}},
		{"a blank block is dropped", "a<think>  </think>b", "ab", nil},
		{"two blocks", "<think>1</think><think>2</think>rest", "rest", []string{"1", "2"}},
		{"the long tag ends only at its own close", "<thinking>x</think>y</thinking>z", "z", []string{"x</think>y"}},
		{"a block with a trailing LF", "<think>x\n</think>\n", "", []string{"x"}},
		{"a broken tag", "<think\n>x", "<think\n>x", nil},
		{"tags are case-sensitive", "<THINK>x</THINK>", "<THINK>x</THINK>", nil},
		{"Unicode spaces are stripped", "\u3000<think>\u3000a\u3000</think>\u3000", "", []string{"a"}},
		{"an unclosed block over lines", "<think>a\nb\n", "", []string{"a\nb"}},
		{"an empty block at the end", "x<think>", "x", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			text, thoughts := splitThink(test.content)
			if text != test.text || !slices.Equal(thoughts, test.thoughts) {
				t.Errorf("splitThink(%q) = %q, %q; want %q, %q", test.content, text, thoughts, test.text, test.thoughts)
			}
		})
	}
}

func TestLogHeaderTakesASCIIDigitsOnly(t *testing.T) {
	line := "2026-10-01 08:00:00.123 [info] x"
	if !logHeader.MatchString(line) {
		t.Fatalf("logHeader does not match %q", line)
	}
	if other := strings.ReplaceAll(line, "3", "\u0663"); logHeader.MatchString(other) {
		t.Errorf("logHeader matches %q", other)
	}
}
