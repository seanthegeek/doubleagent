// Package twinny reads Twinny's conversations, standing grants, file
// changes and log channel.
//
// It reads:
//
//   - User/globalStorage/state.vscdb, the ItemTable row rjmacarthy.twinny:
//     twinny.conversations (keyed by id) and twinny.active-conversation
//     (only when its id is missing from the first), the standing grants
//     under twinny-global-context- and twinny.teamPolicy;
//   - User/workspaceStorage/<id>/state.vscdb: the review tab's owner and
//     repository, one "source host repository:" row;
//   - agent-changes/<uuid>.json, a secondary source repeating the
//     conversation's file changes;
//   - Twinny.log and its rotated copies, secondary: one system row per
//     entry, whose host-local time is read as UTC.
//
// The catalog gives the state database to the editor whose User directory
// holds it, so the parser reads every editor agent's. Messages carry no
// time: every row of a conversation takes its updatedAt, and source_line
// is 0. Each provider holding an API key gives a "model provider:" row with
// the key through parserkit.SecretText. See research/twinny.md.
package twinny
