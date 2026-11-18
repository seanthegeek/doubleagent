package cody

import (
	"testing"
	"time"
	// The zone database is built into the test, so every runner, Windows
	// included, has the same zones.
	_ "time/tzdata"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// TestAMailDateGivesOneTimeInEveryHostZone runs the mail dates under host
// zones that hold their offsets and names. time.Parse reads the host zone
// from time.Local, set once per process, so the test sets time.Local
// itself and puts it back.
func TestAMailDateGivesOneTimeInEveryHostZone(t *testing.T) {
	dates := []struct{ name, text, want string }{
		{"a positive offset", "Mon, 02 Jan 2006 15:04:05 +0100", "2006-01-02T14:04:05.000Z"},
		{"a half-hour offset", "Mon, 02 Jan 2006 15:04:05 +0530", "2006-01-02T09:34:05.000Z"},
		{"a nine-hour offset", "Mon, 02 Jan 2006 15:04:05 +0900", "2006-01-02T06:04:05.000Z"},
		{"a summer offset", "Mon, 02 Jul 2006 15:04:05 +0200", "2006-07-02T13:04:05.000Z"},
		{"a negative offset", "Mon, 02 Jan 2006 15:04:05 -0500", "2006-01-02T20:04:05.000Z"},
		{"an offset with a zone comment", "Mon, 02 Jan 2006 15:04:05 +0100 (CET)", "2006-01-02T14:04:05.000Z"},
		{"GMT", "Mon, 02 Jan 2006 15:04:05 GMT", "2006-01-02T15:04:05.000Z"},
		{"a North American zone name", "Mon, 02 Jan 2006 15:04:05 EST", "2006-01-02T20:04:05.000Z"},
		{"a zone name outside RFC 5322 is UTC", "Mon, 02 Jan 2006 15:04:05 CET", "2006-01-02T15:04:05.000Z"},
		{"a zone name of Asia is UTC", "Mon, 02 Jan 2006 15:04:05 IST", "2006-01-02T15:04:05.000Z"},
	}
	for _, zone := range []string{"UTC", "Europe/Paris", "Asia/Kolkata", "Asia/Tokyo", "America/New_York"} {
		location, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatal(err)
		}
		t.Run("on a host in "+zone, func(t *testing.T) {
			saved := time.Local
			time.Local = location
			t.Cleanup(func() { time.Local = saved })
			for _, date := range dates {
				if got := chatTime(date.text, jsonvalue.NewNull()); got != date.want {
					t.Errorf("%s: chatTime(%q) = %q, want %q", date.name, date.text, got, date.want)
				}
			}
		})
	}
}
