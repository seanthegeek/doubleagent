package regtext

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// initialSize is the first buffer size tried; a longer value is read again
// with the size the registry reports.
const initialSize = 256

// Read returns the text of the value name under key, decoded as Decode does,
// and the value's type. A missing value gives registry.ErrNotExist; a value
// that is neither REG_SZ nor REG_EXPAND_SZ gives its type and
// registry.ErrUnexpectedType. A REG_EXPAND_SZ value is never expanded.
func Read(key registry.Key, name string) (text string, kind uint32, err error) {
	data := make([]byte, initialSize)
	for {
		n, valueKind, err := key.GetValue(name, data)
		if errors.Is(err, syscall.ERROR_MORE_DATA) && n > len(data) {
			// The value grew or was longer than the buffer: read it again
			// with the size the registry reported.
			data = make([]byte, n)
			continue
		}
		if err != nil {
			return "", valueKind, err
		}
		if valueKind != registry.SZ && valueKind != registry.EXPAND_SZ {
			return "", valueKind, registry.ErrUnexpectedType
		}
		return Decode(data[:n]), valueKind, nil
	}
}
