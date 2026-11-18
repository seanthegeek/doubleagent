//go:build parity

package replay

import (
	"regexp"
	"testing"
)

// wantedDifferences returns the differences numbered 240 onward.
func wantedDifferences() []Difference {
	return []Difference{
		{Number: 240, Summary: "OpenHands reads a conversation's meta.json so a damaged one is reported: " +
			"the binary may want a conversation's meta.json, which the recording's parser did not",
			Wanted: wantedOpenHandsMeta},
	}
}

// openHandsMeta matches the home-relative path of an OpenHands
// conversation's meta.json, under each directory the agent keeps its
// conversations in.
var openHandsMeta = regexp.MustCompile(`\A\.openhands/(?:agent-canvas/(?:dev_)?conversations|conversations)/` +
	`[0-9a-fA-F]{32}/meta\.json\z`)

// wantedOpenHandsMeta reports whether the OpenHands parser may want the
// file at rel where the recording did not: a conversation's meta.json.
func wantedOpenHandsMeta(agent, rel string) bool {
	return agent == "openhands" && openHandsMeta.MatchString(rel)
}

// wantsAgree compares one wants call: it agrees when got equals want, or
// when the binary wants a file the recording did not and a difference's
// Wanted rule names the agent and path, and it returns the numbers of the
// differences that let it agree.
func wantsAgree(agent, rel string, got, want bool) (bool, []int) {
	if got == want {
		return true, nil
	}
	if !got {
		return false, nil
	}
	for _, entry := range Differences() {
		if entry.Wanted != nil && entry.Wanted(agent, rel) {
			return true, []int{entry.Number}
		}
	}
	return false, nil
}

func TestDifference240LetsOpenHandsWantOnlyAConversationsMetaFile(t *testing.T) {
	const conversation = "0123456789abcdef0123456789abcdef"
	meta := ".openhands/conversations/" + conversation + "/meta.json"
	tests := []struct {
		name, agent, rel string
		got, want, agree bool
	}{
		{"OpenHands wanting a conversation's meta.json agrees", "openhands", meta, true, false, true},
		{"an agent-canvas conversation's meta.json agrees", "openhands",
			".openhands/agent-canvas/dev_conversations/" + conversation + "/meta.json", true, false, true},
		{"another agent wanting the same path fails", "cline", meta, true, false, false},
		{"OpenHands wanting another file of the conversation fails", "openhands",
			".openhands/conversations/" + conversation + "/events/meta.json", true, false, false},
		{"OpenHands wanting a meta.json outside a conversation fails", "openhands",
			".openhands/conversations/x/meta.json", true, false, false},
		{"OpenHands no longer wanting a file the recording wanted fails", "openhands", meta, false, true, false},
		{"an equal answer agrees with no difference", "openhands", meta, false, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agree, numbers := wantsAgree(test.agent, test.rel, test.got, test.want)
			wantNumbers := 0
			if agree && test.got != test.want {
				wantNumbers = 1
			}
			if agree != test.agree || len(numbers) != wantNumbers {
				t.Errorf("wantsAgree() = %v, %v; want %v", agree, numbers, test.agree)
			}
		})
	}
}
