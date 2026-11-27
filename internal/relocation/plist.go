package relocation

import "context"

// PlistReader reads launchd property lists; hostcmd.Plutil is the one in use.
type PlistReader interface {
	// Available reports whether the reader can run on this host.
	Available() bool
	// Lint reports whether the file at path is a property list it can read.
	Lint(ctx context.Context, path string) bool
	// ExtractString reads the string at keyPath, as hostcmd.Plutil documents.
	ExtractString(ctx context.Context, path, keyPath string) (value, reason string, found bool)
}

// fileValues is what a value reader made of one hit file: a Reason for the
// whole file (`no plutil`, `plutil could not read the file`), or the
// Assignments it read, and for a plist the UserName when it is a plain account
// name.
type fileValues struct {
	Reason      string
	Assignments []assignment
	User        string
}

// isAccountName reports a plain account name of at most maxAccountNameBytes:
// a letter or underscore, then letters, digits, `_`, `.` and `-`.
func isAccountName(name string) bool {
	if name == "" || len(name) > maxAccountNameBytes || !isNameStart(name[0]) {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isWordByte(name[i]) && name[i] != '.' && name[i] != '-' {
			return false
		}
	}
	return true
}

// maxAccountNameBytes is the longest launchd UserName kept as a hint's user:
// 255 bytes, the MAXLOGNAME of macOS's sys/param.h.
const maxAccountNameBytes = 255

// resolvePlist reads, through plutil only, the value of each distinct name of
// hits under the plist's top-level EnvironmentVariables dictionary, as
// launchd reads it, and the plist's UserName. Each value is an assignment on
// line 0, since plutil reports no line; a name with no value there gives
// none. A UserName is kept when it starts with a letter or underscore and
// holds only letters, digits, `_`, `.` and `-`, up to maxAccountNameBytes
// long. Once ctx is cancelled no further plutil is run and ctx's error is
// returned, with no values.
func resolvePlist(ctx context.Context, plutil PlistReader, path string, hits []lineName,
) (fileValues, error) {
	if err := ctx.Err(); err != nil {
		return fileValues{}, err
	}
	if plutil == nil || !plutil.Available() {
		return fileValues{Reason: "no plutil"}, nil
	}
	linted := plutil.Lint(ctx, path)
	if err := ctx.Err(); err != nil {
		return fileValues{}, err
	}
	if !linted {
		return fileValues{Reason: "plutil could not read the file"}, nil
	}
	var values fileValues
	seen := map[string]bool{}
	for _, hit := range hits {
		if seen[hit.Name] {
			continue
		}
		seen[hit.Name] = true
		value, reason, found := plutil.ExtractString(ctx, path, "EnvironmentVariables."+hit.Name)
		if err := ctx.Err(); err != nil {
			return fileValues{}, err
		}
		if found {
			values.Assignments = append(values.Assignments,
				assignment{Line: 0, Name: hit.Name, Value: value, Reason: reason})
		}
	}
	user, reason, found := plutil.ExtractString(ctx, path, "UserName")
	if found && reason == "" && isAccountName(user) {
		values.User = user
	}
	return values, ctx.Err()
}

// valueIndex is a fileValues with the place of the last assignment of each
// line and name, so finding a hint's value costs one lookup, not a scan of
// every assignment, which made a file of many assignments cost their square.
// Work counts the assignments indexed and the lookups made, for the cost test.
type valueIndex struct {
	values fileValues
	last   map[lineName]int
	Work   int
}

// index returns the valueIndex of values, a later assignment of a line and
// name taking the place of an earlier one.
func (values fileValues) index() *valueIndex {
	index := &valueIndex{values: values, last: make(map[lineName]int, len(values.Assignments))}
	for i, assigned := range values.Assignments {
		index.last[lineName{Line: assigned.Line, Name: assigned.Name}] = i
		index.Work++
	}
	return index
}

// selectValue picks the outcome of the hint of name on line: the whole file's
// reason, or the last assignment of that line and name, its value passed
// through acceptValue. Both results are empty when the line assigns no name.
func (index *valueIndex) selectValue(line int, name, root string) (value, reason string) {
	if index.values.Reason != "" {
		return "", index.values.Reason
	}
	index.Work++
	place, found := index.last[lineName{Line: line, Name: name}]
	if !found {
		return "", ""
	}
	outcome := index.values.Assignments[place]
	if tooLong(outcome.Value) {
		return "", reasonTooLong
	}
	if outcome.Reason != "" {
		return "", outcome.Reason
	}
	return acceptValue(outcome.Value, root)
}
