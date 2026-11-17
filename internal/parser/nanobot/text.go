package nanobot

import (
	"encoding/base64"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// textPatterns are the patterns of the `/model <preset>` command and its
// reply (nanobot/command/builtin.py).
type textPatterns struct {
	modelCommand *regexp.Regexp
	switched     *regexp.Regexp
	resolved     *regexp.Regexp
}

// newTextPatterns compiles the patterns. Spaces are the ones Python's \s
// and \S name in text.
func newTextPatterns() textPatterns {
	space, other := `[`+parser.PythonSpace+`]`, `[^`+parser.PythonSpace+`]`
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
	return pytext.Strip(pytext.DecodeUTF8(content))
}

// relayTexts returns the `session relayed:` row texts of a session: one for
// a session key <channel>:<chat id> of a messaging platform, and one for a
// metadata.last_channel route of a messaging platform that differs from
// the key.
func relayTexts(sessionID string, lastChannel pyjson.Value) []string {
	var texts []string
	if channel, chat, _ := strings.Cut(sessionID, ":"); isMessaging(channel) && chat != "" {
		texts = append(texts, "session relayed: "+channel+" chat "+chat+" (from session key)")
	}
	if lastChannel.IsString() && lastChannel.Text() != sessionID {
		if channel, chat, _ := strings.Cut(lastChannel.Text(), ":"); isMessaging(channel) && chat != "" {
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
// session database's rows are rebuilt into: a blob as the text Python's
// str() writes for it, which reads as no time and prints the same.
func columnValue(value sqlitefile.Value) pyjson.Value {
	switch value.Type {
	case sqlitefile.Null:
		return pyjson.NewNull()
	case sqlitefile.Integer:
		return pyjson.NewInt(value.Int)
	case sqlitefile.Float:
		return pyjson.NewFloat(value.Float)
	case sqlitefile.Text:
		return pyjson.NewString(value.Text())
	}
	return pyjson.NewString(value.Str())
}

// firstStr returns the first of the values Python counts as true as its
// str(), or "".
func firstStr(values ...pyjson.Value) string {
	for _, value := range values {
		if value.Truthy() {
			return value.Str()
		}
	}
	return ""
}

// getOr returns the str() of the member key of object, or fallback when the
// object has no such member: Python's "%s" % record.get(key, fallback).
func getOr(object pyjson.Value, key, fallback string) string {
	if value, ok := object.Lookup(key); ok {
		return value.Str()
	}
	return fallback
}

// isText reports whether value is the string text.
func isText(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}
