//go:build parity

package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Row is one timeline row as the Python parser yielded it, in the field
// order of timeline.jsonl.
type Row struct {
	TimestampUTC Text `json:"timestamp_utc"`
	Host         Text `json:"host"`
	User         Text `json:"user"`
	Agent        Text `json:"agent"`
	SessionID    Text `json:"session_id"`
	ProjectPath  Text `json:"project_path"`
	GitBranch    Text `json:"git_branch"`
	TurnType     Text `json:"turn_type"`
	Model        Text `json:"model"`
	ToolName     Text `json:"tool_name"`
	ToolUseID    Text `json:"tool_use_id"`
	Text         Text `json:"text"`
	SourceFile   Text `json:"source_file"`
	SourceLine   int  `json:"source_line"`
	Recovered    Text `json:"recovered"`
}

// Home is a base the artifacts are relative to, as the Python Home holds it.
// StandIn marks a test's stand-in object that had only some of the fields.
type Home struct {
	DiskPath    Text            `json:"disk_path"`
	Original    Text            `json:"original"`
	User        Text            `json:"user"`
	Host        Text            `json:"host"`
	Inferred    bool            `json:"inferred"`
	Project     bool            `json:"project"`
	StagedPaths map[string]Text `json:"staged_paths"`
	StandIn     bool            `json:"stand_in"`
}

// Artifact is one attributed file as a parser was given it. Manifest is the
// manifest row's JSON, or null. In an input case HomeIndex names the
// collection's home and Home is filled in from it.
type Artifact struct {
	DiskPath  Text            `json:"disk_path"`
	Original  Text            `json:"original"`
	Rel       Text            `json:"rel"`
	Agent     Text            `json:"agent"`
	Manifest  json.RawMessage `json:"manifest"`
	Home      *Home           `json:"-"`
	HomeIndex int             `json:"-"`
	StandIn   bool            `json:"stand_in"`
}

// UnmarshalJSON reads an artifact whose home is an object or an index.
func (a *Artifact) UnmarshalJSON(data []byte) error {
	type plain Artifact
	var fields struct {
		plain
		Home json.RawMessage `json:"home"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("artifact: %w", err)
	}
	*a = Artifact(fields.plain)
	a.HomeIndex = -1
	switch home := bytes.TrimSpace(fields.Home); {
	case len(home) == 0 || bytes.Equal(home, []byte("null")):
	case home[0] == '{':
		a.Home = &Home{}
		if err := json.Unmarshal(home, a.Home); err != nil {
			return fmt.Errorf("artifact home: %w", err)
		}
	default:
		if err := json.Unmarshal(home, &a.HomeIndex); err != nil {
			return fmt.Errorf("artifact home index: %w", err)
		}
	}
	return nil
}

// Options are the timeline options a parse was given.
type Options struct {
	IncludeThinking bool `json:"include_thinking"`
}

// CaseError is an exception the Python call raised: its qualified type and
// its text.
type CaseError struct {
	Type Text `json:"type"`
	Text Text `json:"text"`
}

// ParserResult is what a parser call gave: for parse the rows in yield order,
// whether the generator ran to its end, and any exception; for the other
// methods Value (a bool, or a number for secondary_tolerance) or Error.
type ParserResult struct {
	Rows      []Row           `json:"rows"`
	Completed bool            `json:"completed"`
	Value     json.RawMessage `json:"value"`
	Error     *CaseError      `json:"error"`
}

// ParserCase is one call of parse, wants, secondary or secondary_tolerance.
// Instance and Call place it among the calls on one parser object, for the
// parsers that cache across calls. Tree and Times are empty when the artifact
// lies outside the recorder's root (a stand-in carrying only rel).
type ParserCase struct {
	Node     string       `json:"node"`
	Seq      int          `json:"seq"`
	Patched  []string     `json:"patched"`
	Method   string       `json:"method"`
	Agent    string       `json:"agent"`
	Class    string       `json:"class"`
	Module   string       `json:"module"`
	Instance string       `json:"instance"`
	Call     int          `json:"call"`
	Artifact Artifact     `json:"artifact"`
	Options  *Options     `json:"options"`
	Tree     string       `json:"tree"`
	Times    string       `json:"times"`
	Result   ParserResult `json:"result"`
	// ResultBlob names the blob holding Result's JSON for a parse; the
	// loaders fill Result from it.
	ResultBlob string `json:"result_blob"`
}

// CommandCase is one run of the command line. Exit is the return value or
// the SystemExit code as JSON (a number, null, or a message); Outputs maps the
// root-relative path of each file the run created or changed to its blob
// hash, or to {"link": target} for a link.
type CommandCase struct {
	Node    string                     `json:"node"`
	Seq     int                        `json:"seq"`
	Patched []string                   `json:"patched"`
	Argv    []Text                     `json:"argv"`
	Cwd     Text                       `json:"cwd"`
	Clock   []string                   `json:"clock"`
	Tree    string                     `json:"tree"`
	Times   string                     `json:"times"`
	Stdout  Text                       `json:"stdout"`
	Stderr  Text                       `json:"stderr"`
	Exit    json.RawMessage            `json:"exit"`
	Error   *CaseError                 `json:"error"`
	Outputs map[string]json.RawMessage `json:"outputs"`
	Removed []string                   `json:"removed"`
}

// Collection is what open_input returned.
type Collection struct {
	Kind      Text            `json:"kind"`
	Source    Text            `json:"source"`
	Root      Text            `json:"root"`
	Host      Text            `json:"host"`
	Notes     []Text          `json:"notes"`
	Problems  []Text          `json:"problems"`
	Summary   json.RawMessage `json:"summary"`
	Homes     []Home          `json:"homes"`
	Artifacts []Artifact      `json:"artifacts"`
}

// InputCase is one call of open_input: its arguments, the tree, and the
// collection or the exception.
type InputCase struct {
	Node    string      `json:"node"`
	Seq     int         `json:"seq"`
	Patched []string    `json:"patched"`
	Path    Text        `json:"path"`
	WorkDir *Text       `json:"work_dir"`
	Host    Text        `json:"host"`
	Tree    string      `json:"tree"`
	Times   string      `json:"times"`
	Result  *Collection `json:"result"`
	// ResultBlob names the blob holding Result's JSON; InputCases fills
	// Result from it.
	ResultBlob string     `json:"result_blob"`
	Error      *CaseError `json:"error"`
}

// resolveHomes points each artifact that names its home by index at that
// home of the collection.
func (c *InputCase) resolveHomes() error {
	if c.Result == nil {
		return nil
	}
	for i := range c.Result.Artifacts {
		artifact := &c.Result.Artifacts[i]
		if artifact.Home != nil {
			continue
		}
		if artifact.HomeIndex < 0 || artifact.HomeIndex >= len(c.Result.Homes) {
			return fmt.Errorf("artifact %q names home %d of %d", artifact.Rel, artifact.HomeIndex,
				len(c.Result.Homes))
		}
		artifact.Home = &c.Result.Homes[artifact.HomeIndex]
	}
	return nil
}
