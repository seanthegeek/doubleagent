// Package dedup drops the rows of secondary sources that a primary store
// also records. A secondary source is a file a parser reads beside the
// agent's own transcript store that repeats some of its events: a
// per-prompt history, a mirror or a projection. Every source is parsed on
// every run; this package keeps each event once.
//
// A secondary row is dropped when a primary row has the same identity
// (host, user, agent, turn type and text, or the same tool_use_id for two
// tool rows that both carry one), the same scope (session, else project,
// else none) and a time within the source's tolerance. A secondary row with
// no time matches on identity and scope alone. Matching is one-to-one,
// timestamped rows first, each taking the nearest unused candidate, the
// earlier one on a tie. The text is the row's text as it reads in full,
// every shared text it names in place (timeline.SharedText), read once
// however many rows name it; texts are equal only byte for byte (Texts).
//
// The secondary rows no primary row took are then matched against each
// other, looking under broader scopes too, so the several copies of one
// event that backups and mirrors keep are shown once. A row pairs only with
// a row from a different source file, and a parser: row is never dropped
// or taken. Every secondary row kept that is not a system row is marked
// Recovered "yes".
//
// An agent whose secondary source writes events differently from its
// primary store declares AgentHooks: another match form, a wider forward
// window for some rows, or the time a row is matched at. A widened row may
// not take a primary row at or after the next secondary row of the same
// text that could take that primary row, so the later line keeps the later
// record.
package dedup
