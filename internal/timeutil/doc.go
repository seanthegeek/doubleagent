// Package timeutil normalises timestamps to the one form the timeline
// writes, YYYY-MM-DDTHH:MM:SS.mmmZ: UTC, milliseconds cut (never rounded),
// so rows from different agents sort together as plain strings. A value
// that does not read as a time gives "", never an error, so one bad record
// never stops a timeline. Years outside 1 to 9999 give "".
//
// Parsers call FromValue on a decoded JSON field, FromText on text,
// FromEpoch on a float and FromInt on an integer. A number, or text of
// ASCII digits with at most one point, is an epoch: milliseconds when it
// is above 1e11, else seconds. An epoch is read from its decimal digits,
// exactly, and cut to the millisecond. Other text is a date, or a date and
// time, in one of these forms, read through time.Parse layouts:
//
//	YYYY-MM-DD
//	YYYY-MM-DDTHH:MM    YYYY-MM-DDTHH:MM:SS    YYYY-MM-DDTHH:MM:SS.fffffffff
//	any of these time forms followed by Z, +HH:MM, +HHMM or +HH (or a minus)
//
// The fraction may have any number of digits, after a point or a comma;
// T, a lower-case t or one space separates date and time; a lower-case z
// is Z; no zone is UTC. The hour has two digits and an offset's hours run
// to 23. Anything else gives "": week and ordinal dates, a basic date and
// time without separators (a basic date alone is all digits, so an epoch),
// any other separator, a date with an offset but no time, 24:00, a second
// of 60, an offset with seconds, non-ASCII digits.
//
// time.Parse with time.RFC3339 reads one form only and needs seconds and
// an offset; agents also write dates alone, times without seconds or zone,
// a space separator and epoch numbers, so the forms above are tried in turn
// and an epoch is read from its digits.
package timeutil
