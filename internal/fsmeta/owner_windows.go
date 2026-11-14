package fsmeta

import (
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// Owners names the owners of files as the PowerShell collector's
// Get-OwnerName does, looking each owner up once per run. It is not safe for
// use by several goroutines at once.
type Owners struct {
	names map[string]string // security identifier text -> account name
}

// NewOwners returns an Owners with an empty cache, for one run.
func NewOwners() *Owners {
	return &Owners{names: map[string]string{}}
}

// Name returns the owner of path itself (a link's own owner, never its
// target's) as DOMAIN\name, or the account name alone when it has no
// domain, or the security identifier text (S-1-5-...) when no account
// matches it, or "" when the security descriptor cannot be read.
func (o *Owners) Name(path string) string {
	owner, err := ownerOf(path)
	if err != nil {
		return ""
	}
	key := owner.String()
	if name, ok := o.names[key]; ok {
		return name
	}
	name := key
	account, domain, _, err := owner.LookupAccount("")
	switch {
	case err != nil:
	case domain == "":
		name = account
	default:
		name = domain + `\` + account
	}
	o.names[key] = name
	return name
}

// ownerOf reads the owner security identifier of path itself, through a
// handle opened with only the right to read its security descriptor.
func ownerOf(path string) (*windows.SID, error) {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.READ_CONTROL, shareAll, nil,
		windows.OPEN_EXISTING, noFollowFlags, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return nil, err
	}
	owner, _, err := descriptor.Owner()
	return owner, err
}
