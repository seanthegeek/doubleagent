package collect

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// probeWriteThen is probeWrite with a call, when set, between the probe's
// creation and its removal, given the probe's name. The probe is created
// by its path as the run's own files are (named under the canonical dir
// as the output names are; CREATE_NEW, without the backup intent an
// os.Root open adds), with FILE_FLAG_DELETE_ON_CLOSE, so closing it
// removes the very file made and no name is looked up a second time; the
// DELETE access that needs makes a directory that allows creating files
// but not deleting them not writable, where the run could not finish
// anyway: its partial archive is renamed and its staging directory removed.
func probeWriteThen(dir string, created func(probe string)) bool {
	random := make([]byte, 16)
	_, _ = rand.Read(random)
	probe := ".write-probe-" + hex.EncodeToString(random)
	name, err := windows.UTF16PtrFromString(syspath.Extended(dir + "/" + probe))
	if err != nil {
		return false
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE|windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT|
			windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err != nil {
		return false
	}
	if created != nil {
		created(probe)
	}
	_ = windows.CloseHandle(handle)
	return true
}
