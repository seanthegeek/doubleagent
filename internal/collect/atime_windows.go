package collect

import "golang.org/x/sys/windows/registry"

// fileSystemKey holds NtfsDisableLastAccessUpdate, the system-wide setting
// that fsutil behavior query disablelastaccess shows.
const fileSystemKey = `SYSTEM\CurrentControlSet\Control\FileSystem`

// logsAccessDecision says the start of the log says how files are opened:
// on Windows the copy's open depends on the setting and the option.
const logsAccessDecision = true

// lastAccessUpdates reads whether this system updates last-access times.
func lastAccessUpdates() accessUpdates {
	state, _, _ := readLastAccessSetting()
	return state
}

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
