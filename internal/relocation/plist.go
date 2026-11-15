package relocation

import "strings"

// PlistReader reads launchd property lists; hostcmd.Plutil is the one in use.
type PlistReader interface {
	// Available reports whether the reader can run on this host.
	Available() bool
	// Lint reports whether the file at path is a property list it can read.
	Lint(path string) bool
	// ExtractString reads the string at keyPath, as hostcmd.Plutil documents.
	ExtractString(path, keyPath string) (value, reason string, found bool)
}

// FileValues is what a value reader made of one hit file: a Reason for the
// whole file (`no plutil`, `plutil could not read the file`), or the
// Assignments it read, and for a plist the UserName when it is a plain account
// name.
type FileValues struct {
	Reason      string
	Assignments []Assignment
	User        string
}

// ResolvePlist reads, through plutil only, the value of each distinct name of
// hits under the plist's top-level EnvironmentVariables dictionary, as
// launchd reads it, and the plist's UserName. Each value is an Assignment on
// line 0, since plutil reports no line; a name with no value there gives
// none. A UserName is kept when it starts with a letter or underscore and
// holds only letters, digits, `_`, `.` and `-`.
func ResolvePlist(plutil PlistReader, path string, hits []LineName) FileValues {
	if plutil == nil || !plutil.Available() {
		return FileValues{Reason: "no plutil"}
	}
	if !plutil.Lint(path) {
		return FileValues{Reason: "plutil could not read the file"}
	}
	var values FileValues
	seen := map[string]bool{}
	for _, hit := range hits {
		if seen[hit.Name] {
			continue
		}
		seen[hit.Name] = true
		value, reason, found := plutil.ExtractString(path, "EnvironmentVariables."+hit.Name)
		if found {
			values.Assignments = append(values.Assignments,
				Assignment{Line: 0, Name: hit.Name, Value: value, Reason: reason})
		}
	}
	user, reason, found := plutil.ExtractString(path, "UserName")
	if found && reason == "" && isAccountName(user) {
		values.User = user
	}
	return values
}

// isAccountName reports a plain account name: a letter or underscore, then
// letters, digits, `_`, `.` and `-`.
func isAccountName(name string) bool {
	if name == "" || !isNameStart(name[0]) {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isWordByte(name[i]) && name[i] != '.' && name[i] != '-' {
			return false
		}
	}
	return true
}

// selectValue picks the outcome of the hint of name on line: the whole file's
// reason, or the last Assignment of that line and name, its value passed
// through AcceptValue. Both results are empty when the line assigns no name.
func (values FileValues) selectValue(line int, name, root string) (value, reason string) {
	if values.Reason != "" {
		return "", values.Reason
	}
	var outcome *Assignment
	for i := range values.Assignments {
		if values.Assignments[i].Line == line && values.Assignments[i].Name == name {
			outcome = &values.Assignments[i]
		}
	}
	if outcome == nil {
		return "", ""
	}
	if outcome.Reason != "" {
		return "", outcome.Reason
	}
	return AcceptValue(readField(outcome.Value), root)
}

// readField reproduces how the sh collector's `IFS='|' read` hands a value
// over: as the last field it keeps the rest of the record, except that a
// single trailing `|` is dropped when the value holds no other one.
func readField(value string) string {
	if trimmed, cut := strings.CutSuffix(value, "|"); cut && !strings.Contains(trimmed, "|") {
		return trimmed
	}
	return value
}
