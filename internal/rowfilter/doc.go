// Package rowfilter filters the timeline's rows by a time window (--since,
// --until) and by text (--match).
//
// A --since or --until value is one of the WHEN forms FormsHelp lists: an
// ISO 8601 date, a year or month, a date and time with T or one space
// between them and an optional offset (directly after the time, or after
// one space, where UTC also means Z); epoch seconds or milliseconds;
// today, yesterday or now; or a time relative to now (45m, 36h, 3d, 2w,
// 6mo, 1y, optionally followed by ago), months and years moving the
// calendar date and keeping the time of day. Words, units, Z and UTC may be
// in any ASCII case. A value without an offset is UTC. A value naming a
// period (a year, a month, a day, an hour, a minute, a second) gives its
// first millisecond for --since and its last for --until. Anything else is
// a BoundError; nothing is guessed. Dates may be ISO week dates or ordinal
// dates as well. A lower-case t or another separator, an offset straight
// after a date, a comma before the fraction, 24:00, fields of one digit and
// non-ASCII digits are refused.
//
// Bounds are timestamps in the timeline's own form, so a row is kept by a
// plain string comparison: since <= timestamp_utc <= until.
//
// time.ParseDuration reads units from nanoseconds to hours, with no days,
// weeks, months or years and no calendar arithmetic, and time.Parse reads
// one layout at a time, so the WHEN forms are read here.
package rowfilter
