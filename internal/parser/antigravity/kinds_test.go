package antigravity

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// countOf returns how many of the rows have exactly the given text.
func countOf(rows []string, text string) int {
	total := 0
	for _, row := range rows {
		if row == text {
			total++
		}
	}
	return total
}

// memberCase is one member of a file the format makes an array or an
// object: the file, the member's name in the problem, its kind, a value of
// the other container kind, and the file's text built around a value (the
// member left out for an empty value).
type memberCase struct {
	rel, member string
	kind        jsonvalue.Kind
	other       string
	build       func(value string) string
}

// checkMember checks the problem row appears once for a string, a number
// and a value of the other container kind, and not for null, an empty
// value or an absent member.
func checkMember(t *testing.T, test memberCase) {
	t.Helper()
	problem := parser.ProblemPrefix + parser.WrongKind(test.member, test.kind)
	empty := `[]`
	if test.kind == jsonvalue.Object {
		empty = `{}`
	}
	cases := []struct {
		name, json string
		want       int
	}{
		{"a string", `"oops"`, 1}, {"a number", `7`, 1}, {"the other kind", test.other, 1},
		{"null", `null`, 0}, {"empty", empty, 0}, {"absent", "", 0},
	}
	for _, one := range cases {
		t.Run(test.member+" "+one.name, func(t *testing.T) {
			if got := countOf(parseFile(t, test.rel, []byte(test.build(one.json))), problem); got != one.want {
				t.Errorf("%d rows %q, want %d", got, problem, one.want)
			}
		})
	}
}

// field returns `"name": value,` or nothing for no value.
func field(name, value string) string {
	if value == "" {
		return ""
	}
	return `"` + name + `": ` + value + `, `
}

func TestABrainLinesToolCallsThatIsNotAnArrayIsReported(t *testing.T) {
	checkMember(t, memberCase{
		rel:    ".gemini/antigravity/brain/c1/.system_generated/logs/transcript.jsonl",
		member: "tool_calls", kind: jsonvalue.Array, other: `{"name": "x"}`,
		build: func(value string) string {
			return `{"type": "PLANNER_RESPONSE", ` + field("tool_calls", value) + `"content": "hi"}` + "\n"
		}})
}

func TestAStandingFileMemberOfTheWrongKindIsReported(t *testing.T) {
	const (
		config  = ".gemini/config/config.json"
		project = ".gemini/config/projects/p.json"
		sidecar = ".gemini/config/sidecars/s/sidecar.json"
		cli     = ".gemini/antigravity-cli/settings.json"
	)
	wrap := func(name string) func(value string) string {
		return func(value string) string { return `{` + field(name, value) + `"x": 1}` }
	}
	nested := func(outer, name string) func(value string) string {
		return func(value string) string { return `{"` + outer + `": {` + field(name, value) + `"x": 1}}` }
	}
	cases := []memberCase{
		{config, "userSettings", jsonvalue.Object, `[1]`, wrap("userSettings")},
		{config, "userSettings.globalPermissionGrants", jsonvalue.Object, `[1]`,
			nested("userSettings", "globalPermissionGrants")},
		{config, "sidecars", jsonvalue.Object, `[1]`, wrap("sidecars")},
		{config, "sidecars.a", jsonvalue.Object, `[1]`, nested("sidecars", "a")},
		{project, "permissionGrants", jsonvalue.Object, `[1]`, wrap("permissionGrants")},
		{project, "permissionGrants.permissionGrants", jsonvalue.Object, `[1]`,
			nested("permissionGrants", "permissionGrants")},
		{sidecar, "agent_permissions", jsonvalue.Object, `[1]`, wrap("agent_permissions")},
		{sidecar, "agent_permissions.access_grants", jsonvalue.Array, `{"a": 1}`,
			nested("agent_permissions", "access_grants")},
		{cli, "permissions", jsonvalue.Object, `[1]`, wrap("permissions")},
		{cli, "permissions.allow", jsonvalue.Array, `{"a": 1}`, nested("permissions", "allow")},
		{cli, "permissions.deny", jsonvalue.Array, `{"a": 1}`, nested("permissions", "deny")},
		{cli, "trustedWorkspaces", jsonvalue.Array, `{"a": 1}`, wrap("trustedWorkspaces")},
		{config, "userSettings.globalPermissionGrants.allow", jsonvalue.Array, `{"a": 1}`,
			func(value string) string {
				return `{"userSettings": {"globalPermissionGrants": {` + field("allow", value) + `"x": 1}}}`
			}},
		{config, "userSettings.globalPermissionGrants.deny", jsonvalue.Array, `{"a": 1}`,
			func(value string) string {
				return `{"userSettings": {"globalPermissionGrants": {` + field("deny", value) + `"x": 1}}}`
			}},
		{config, "sidecars.a.resolvedPermissions", jsonvalue.Object, `[1]`,
			func(value string) string {
				return `{"sidecars": {"a": {` + field("resolvedPermissions", value) + `"x": 1}}}`
			}},
		{config, "sidecars.a.resolvedPermissions.accessGrants", jsonvalue.Array, `{"a": 1}`,
			func(value string) string {
				return `{"sidecars": {"a": {"resolvedPermissions": {` + field("accessGrants", value) + `"x": 1}}}}`
			}},
		{project, "permissionGrants.permissionGrants.allow", jsonvalue.Array, `{"a": 1}`,
			func(value string) string {
				return `{"permissionGrants": {"permissionGrants": {` + field("allow", value) + `"x": 1}}}`
			}},
		{project, "permissionGrants.permissionGrants.deny", jsonvalue.Array, `{"a": 1}`,
			func(value string) string {
				return `{"permissionGrants": {"permissionGrants": {` + field("deny", value) + `"x": 1}}}`
			}},
	}
	for _, test := range cases {
		checkMember(t, test)
	}
}
