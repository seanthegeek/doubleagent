// Package regtext reads the text of a REG_SZ or REG_EXPAND_SZ registry value
// as .NET's RegistryKey.GetValue returns it: every UTF-16 code unit the value
// holds, embedded NULs (zero code units) included, less one terminating NUL.
// The Win32 string helpers stop at the first NUL instead, which would turn a
// value such as C:\a<NUL>junk into the different, acceptable value C:\a.
package regtext
