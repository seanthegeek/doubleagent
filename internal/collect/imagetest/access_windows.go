package imagetest

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// rightBits returns the access mask bits of one icacls right name a deny
// attribute takes: RD lists a directory (or reads a file's data), WD creates
// a file in it (or writes data), AD creates a subdirectory (or appends data).
func rightBits(name string) (windows.ACCESS_MASK, bool) {
	switch name {
	case "RD":
		return windows.FILE_LIST_DIRECTORY, true
	case "WD":
		return 0x2, true
	case "AD":
		return 0x4, true
	}
	return 0, false
}

// rightsMask returns the access mask of a deny value: icacls names joined by
// commas, such as "WD,AD".
func rightsMask(rights string) (windows.ACCESS_MASK, error) {
	var mask windows.ACCESS_MASK
	for _, name := range strings.Split(rights, ",") {
		bits, known := rightBits(name)
		if !known {
			return 0, fmt.Errorf("deny=%s: %q is not one of RD, WD and AD", rights, name)
		}
		mask |= bits
	}
	return mask, nil
}

// denyAccess adds a rule denying the current user rights on path alone, not
// on anything below it, as icacls /deny does without inheritance flags. It
// returns what puts the access list back as it was.
func denyAccess(path, rights string) (undo func() error, err error) {
	mask, err := rightsMask(rights)
	if err != nil {
		return nil, err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	native := syspath.Extended(path)
	descriptor, err := windows.GetNamedSecurityInfo(native, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return nil, err
	}
	original, _, err := descriptor.DACL()
	if err != nil {
		return nil, err
	}
	denied, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: mask, AccessMode: windows.DENY_ACCESS, Inheritance: windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)},
	}}, original)
	if err != nil {
		return nil, err
	}
	if err := setAccessList(native, denied); err != nil {
		return nil, err
	}
	return func() error { return setAccessList(native, original) }, nil
}

// setAccessList replaces the access list of the file or directory at path.
func setAccessList(path string, list *windows.ACL) error {
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION,
		nil, nil, list, nil)
}

// makeJunction makes link a junction to the directory target, through
// FSCTL_SET_REPARSE_POINT on a fresh directory, as mklink /J does.
func makeJunction(target, link string) error {
	if err := osfile.MkdirAll(link, 0o755); err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(syspath.Extended(link))
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	data := mountPointData(`\??\`+target+`\`, target)
	var returned uint32
	return windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0], uint32(len(data)),
		nil, 0, &returned, nil)
}

// mountPointData lays out a REPARSE_DATA_BUFFER holding a mount-point
// (junction) reparse buffer: the tag, the data length, a reserved word, the
// offsets and lengths in bytes of the two names, then the substitute name
// the system follows and the print name shown to people, each ending in NUL.
func mountPointData(substitute, printName string) []byte {
	substituteText := utf16.Encode([]rune(substitute))
	printText := utf16.Encode([]rune(printName))
	names := make([]uint16, 0, len(substituteText)+len(printText)+2)
	names = append(append(names, substituteText...), 0)
	names = append(append(names, printText...), 0)
	substituteBytes := uint16(2 * len(substituteText))
	data := binary.LittleEndian.AppendUint32(nil, windows.IO_REPARSE_TAG_MOUNT_POINT)
	data = binary.LittleEndian.AppendUint16(data, uint16(8+2*len(names)))
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes+2)
	data = binary.LittleEndian.AppendUint16(data, uint16(2*len(printText)))
	for _, unit := range names {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return data
}
