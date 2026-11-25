// Package users finds the home directories to collect. An image run follows
// one rule on every platform: the image's etc/passwd, then the home parents,
// system and shared homes dropped by their path, -u ignoring case. A live
// run reads, on Unix, the password database and dscl, then the home parents;
// on Windows the registry's ProfileList, then Users and home on the system
// drive.
package users
