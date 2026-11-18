package nanobot

import (
	"encoding/base64"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// textPatterns are the patterns of the `/model <preset>` command and its
// reply (nanobot/command/builtin.py).
type textPatterns struct {
	modelCommand *regexp.Regexp
	switched     *regexp.Regexp
	resolved     *regexp.Regexp
}

// newTextPatterns compiles the patterns. A space is any character of
// parser.Space.
func newTextPatterns() textPatterns {
	space, other := `[`+parser.Space+`]`, `[^`+parser.Space+`]`
	return textPatterns{
		modelCommand: regexp.MustCompile(`(?s)\A/model` + space + `+(` + other + `.*?)` + space + `*\z`),
		switched:     regexp.MustCompile("\\ASwitched model preset to `([^`]*)`"),
		resolved:     regexp.MustCompile("(?m)^- Model: `([^`]*)`"),
	}
}

// messagingChannels are the session key prefixes of the channels that carry
// a chat through a messaging platform (each channel's name,
// nanobot/channels/*/runtime.py at 909b277); websocket (the WebUI), cli,
// api, heartbeat, dream: and unified:default are local.
func messagingChannels() []string {
	return []string{"dingtalk", "discord", "email", "feishu", "linear", "matrix", "mattermost",
		"mochat", "msteams", "napcat", "qq", "signal", "slack", "telegram", "wecom", "weixin",
		"whatsapp", "whatsapp_agent"}
}

// DecodeStem returns the session key a session file's stem names: the stem
// is the key in URL-safe base64 without padding. A stem that is not the
// exact encoding of a UTF-8 key, such as the legacy lossy
// <channel>_<chat> form, is returned unchanged.
func DecodeStem(stem string) string {
	key, err := base64.RawURLEncoding.DecodeString(stem)
	if err != nil || !utf8.Valid(key) || base64.RawURLEncoding.EncodeToString(key) != stem {
		return stem
	}
	return string(key)
}

// readMarker returns the stripped text of a .workspace file, "" when it is
// absent, not a regular file, or cannot be read; a marker that exists but
// cannot be read adds its unreadable problem for the caller to report.
func readMarker(path string, problems *parser.Problems) string {
	if !parser.RegularFile(path) {
		return ""
	}
	content, ok := parser.FileBytes(path, problems)
	if !ok {
		return ""
	}
	return strings.TrimSpace(textlines.ValidUTF8(content))
}

// relayTexts returns the `session relayed:` row texts of a session: one for
// a session key <channel>:<chat id> of a messaging platform, and one for a
// metadata.last_channel route of a messaging platform that differs from
// the key.
func relayTexts(sessionID string, lastChannel jsonvalue.Value) []string {
	var texts []string
	if channel, chat, _ := strings.Cut(sessionID, ":"); isMessaging(channel) && chat != "" {
		texts = append(texts, "session relayed: "+channel+" chat "+chat+" (from session key)")
	}
	if lastChannel.IsString() && lastChannel.StringValue() != sessionID {
		if channel, chat, _ := strings.Cut(lastChannel.StringValue(), ":"); isMessaging(channel) && chat != "" {
			texts = append(texts, "session relayed: "+channel+" chat "+chat+
				" (from metadata.last_channel)")
		}
	}
	return texts
}

// isMessaging reports whether channel is a messaging platform.
func isMessaging(channel string) bool {
	return slices.Contains(messagingChannels(), channel)
}

// columnValue returns a column value as the JSON value of the record the
// session database's rows are rebuilt into: a blob as its row text
// (sqlitefile.Value.String), which reads as no time.
func columnValue(value sqlitefile.Value) jsonvalue.Value {
	switch value.Type {
	case sqlitefile.Null:
		return jsonvalue.NewNull()
	case sqlitefile.Integer:
		return jsonvalue.NewInt(value.Int)
	case sqlitefile.Float:
		return jsonvalue.NewFloat(value.Float)
	case sqlitefile.Text:
		return jsonvalue.NewString(value.Text())
	}
	return jsonvalue.NewString(value.String())
}

// getOr returns the row text of the member key of object, or fallback when
// the object has no such member (a member that is null gives "").
func getOr(object jsonvalue.Value, key, fallback string) string {
	if value, ok := object.Lookup(key); ok {
		return value.Text()
	}
	return fallback
}
