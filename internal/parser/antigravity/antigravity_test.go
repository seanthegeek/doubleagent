package antigravity

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
)

// varintField returns a varint field in wire form.
func varintField(number, value uint64) []byte {
	return append(varint(number<<3), varint(value)...)
}

// bytesField returns a length-delimited field in wire form.
func bytesField(number uint64, data []byte) []byte {
	out := append(varint(number<<3|2), varint(uint64(len(data)))...)
	return append(out, data...)
}

// varint returns value as a protobuf varint.
func varint(value uint64) []byte {
	var out []byte
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

// decodedAs decodes data as the named message, failing the test when it
// does not decode.
func decodedAs(t *testing.T, data []byte, name string) message {
	t.Helper()
	decoded, ok := New().decodeMessage(data, name)
	if !ok {
		t.Fatalf("%s does not decode", name)
	}
	return decoded
}

func TestTheSchemaGivesTheStepKindsInDeclarationOrder(t *testing.T) {
	created := New()
	if created.schemaErr != nil {
		t.Fatal(created.schemaErr)
	}
	want := strings.Fields("code_action finish grep_search view_file list_directory user_input planner_response " +
		"write_to_file error_message run_command checkpoint error find command_status memory read_url_content " +
		"search_web mcp_tool task_boundary notify_user file_change ephemeral_message conversation_history " +
		"send_command_input system_message agency_tool_call shell_exec generic invoke_subagent ask_question " +
		"directory_rules")
	if !slices.Equal(created.payloadKinds, want) {
		t.Errorf("payloadKinds = %v\nwant %v", created.payloadKinds, want)
	}
}

func TestToolArgsSummaryPicksTheNamingArgument(t *testing.T) {
	for _, test := range []struct{ arguments, want string }{
		{`{"CommandLine":"ls","Cwd":"/x"}`, "ls"},
		{`{"Other":1}`, `{"Other":1}`},
		{"not json", "not json"},
		{"", "{}"},
		{`{"Command":"","Path":[1]}`, "[1]"},
		{`{"Command":false}`, "false"},
		{`{"Command":0,"Path":"p"}`, "0"},
		{`{"Command":[],"Path":{},"Url":null,"Name":"n"}`, "n"},
		{`[1, "a"]`, `[1,"a"]`},
		{`"text"`, `"text"`},
		{`{"b":"é","a":1}`, `{"b":"é","a":1}`},
		{"5", "5"},
	} {
		t.Run(test.arguments, func(t *testing.T) {
			if got := toolArgsSummary(test.arguments); got != test.want {
				t.Errorf("toolArgsSummary(%q) = %q, want %q", test.arguments, got, test.want)
			}
		})
	}
}

func TestPermissionTextsNameEachGrant(t *testing.T) {
	grant := slices.Concat(varintField(2, 1), varintField(3, 9), bytesField(5, []byte("a")), bytesField(6, []byte("t")))
	access := slices.Concat(bytesField(1, []byte("file:///home/u/x")), varintField(4, 1), varintField(5, 1))
	permissions := decodedAs(t, slices.Concat(bytesField(2, grant), bytesField(1, access)), "TrajectoryPermissions")
	want := []string{
		"permission grant: action=a target=t allow=true scope=9",
		"permission grant: path=/home/u/x is_directory=false allow=false scope=once from_current_step=true",
	}
	if got := permissionTexts(permissions); !slices.Equal(got, want) {
		t.Errorf("permissionTexts = %q\nwant %q", got, want)
	}
}

func TestDeleteFilePathIsField1(t *testing.T) {
	uri := []byte("file:///home/u/proj/old.log")
	spec := bytesField(4, bytesField(1, bytesField(5, uri)))
	payload := decodedAs(t, slices.Concat(bytesField(1, spec), bytesField(26, []byte("drop it"))), "CortexStepCodeAction")
	if got, want := describePayload("code_action", payload), "delete_file | /home/u/proj/old.log | drop it"; got != want {
		t.Errorf("describePayload = %q, want %q", got, want)
	}
}

func TestChunksSortByTheirNumber(t *testing.T) {
	names := []string{"10", "9", "0010", "1", "01"}
	slices.SortStableFunc(names, parser.CompareDigits)
	if want := []string{"1", "01", "9", "10", "0010"}; !slices.Equal(names, want) {
		t.Errorf("sorted = %v, want %v", names, want)
	}
}

func FuzzStepRendering(fuzz *testing.F) {
	fuzz.Add(slices.Concat(varintField(1, 21), bytesField(28, slices.Concat(bytesField(1, []byte("ls")),
		varintField(6, 2), varintField(14, 1)))))
	created := New()
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		step, ok := created.decodeMessage(data, "Step")
		if !ok {
			return
		}
		for _, kind := range created.payloadKinds {
			_ = describePayload(kind, step.child(kind))
		}
		_ = permissionTexts(step.child("permissions"))
	})
}
