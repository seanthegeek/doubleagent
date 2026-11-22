// Package shortname gives Windows tests a directory typed through its 8.3
// short name: the MS-DOS alias such as PROGRA~1 that NTFS can keep beside a
// long name, which the code that compares paths must respell as the long
// name. Only test files import it, so the binary never links it; on other
// platforms the package is empty.
package shortname
