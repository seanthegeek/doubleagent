package collect

import "golang.org/x/sys/windows/registry"

// fileSystemKey holds NtfsDisableLastAccessUpdate, the system-wide setting
// for last-access updates.
const fileSystemKey = `SYSTEM\CurrentControlSet\Control\FileSystem`

// logsAccessDecision is true: on Windows how a file is opened depends on the
// setting and the option, so the start of the log says which.
const logsAccessDecision = true

// readLastAccessSetting reads NtfsDisableLastAccessUpdate, opening the key
// for reading values alone, and returns its state, its raw value and
// whether a value was read.
func readLastAccessSetting() (accessUpdates, uint64, bool) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, fileSystemKey, registry.QUERY_VALUE)
	if err != nil {
		return accessUnknown, 0, false
	}
	defer key.Close()
	value, valueType, err := key.GetIntegerValue("NtfsDisableLastAccessUpdate")
	if err != nil {
		return accessUnknown, 0, false
	}
	return accessUpdatesOf(value, valueType == registry.DWORD), value, true
}

// lastAccessUpdates reads whether this system updates last-access times.
func lastAccessUpdates() accessUpdates {
	state, _, _ := readLastAccessSetting()
	return state
}
