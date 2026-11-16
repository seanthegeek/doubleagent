package follow

import (
	"fmt"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// NewConfinement returns the confinement of an image run under root, which
// report receives refused paths for, or an error when the root's own place
// cannot be asked: refusing every open would then lose the whole image, so
// the caller runs without the confinement and says so.
func NewConfinement(root string, report func(path string)) (*Confinement, error) {
	handle, err := osfile.Open(root)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	place, err := finalPlace(handle.Fd())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", syspath.ToNative(root), err)
	}
	return newConfinement(place, finalPlace, report), nil
}

// finalPlace returns the NT device path of handle's file, its links
// resolved and each component spelled as the disk holds it
// (GetFinalPathNameByHandle with VOLUME_NAME_NT and FILE_NAME_NORMALIZED).
// The NT form exists for every volume: the DOS form fails for one without a
// drive letter and the GUID form for a network share or a volume the Mount
// Manager did not create.
func finalPlace(handle uintptr) (string, error) {
	// x/sys does not name the flags; Microsoft's page gives
	// VOLUME_NAME_NT = 0x2 and FILE_NAME_NORMALIZED = 0x0.
	const volumeNameNT, fileNameNormalized = 0x2, 0x0
	const flags = volumeNameNT | fileNameNormalized
	buffer := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetFinalPathNameByHandle(windows.Handle(handle), &buffer[0],
			uint32(len(buffer)), flags)
		if err != nil {
			return "", err
		}
		if int(n) < len(buffer) {
			return windows.UTF16ToString(buffer[:n]), nil
		}
		buffer = make([]uint16, n)
	}
}
