package relocation

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// recordingPlist is a PlistReader that records each plutil run it is asked
// for and can cancel the run's context during one of them.
type recordingPlist struct {
	calls    []string
	cancelOn string
	cancel   context.CancelFunc
}

// Available reports that plutil is there; it runs nothing.
func (plist *recordingPlist) Available() bool { return true }

// Lint records the call and accepts the file.
func (plist *recordingPlist) Lint(_ context.Context, path string) bool {
	plist.record("lint " + path)
	return true
}

// ExtractString records the call and finds a string at every key path.
func (plist *recordingPlist) ExtractString(_ context.Context, path, keyPath string) (string,
	string, bool,
) {
	plist.record("extract " + keyPath + " " + path)
	return "svc", "", true
}

// record keeps call and cancels the context when call is the one named.
func (plist *recordingPlist) record(call string) {
	plist.calls = append(plist.calls, call)
	if call == plist.cancelOn {
		plist.cancel()
	}
}

func TestResolvePlistRunsNoPlutilOnceCancelled(t *testing.T) {
	const path = "/p.plist"
	hits := []lineName{{Line: 2, Name: "CODEX_HOME"}}
	allCalls := []string{"lint /p.plist", "extract EnvironmentVariables.CODEX_HOME /p.plist",
		"extract UserName /p.plist"}
	cases := []struct {
		name      string
		cancelOn  string
		cancelled bool
		wantCalls []string
		wantError error
	}{
		{"a live run reads the variable and the user", "", false, allCalls, nil},
		{"a cancelled run runs no plutil", "", true, nil, context.Canceled},
		{"a cancel during the lint runs nothing after it", allCalls[0], false, allCalls[:1],
			context.Canceled},
		{"a cancel during a value read stops before the user", allCalls[1], false, allCalls[:2],
			context.Canceled},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if testCase.cancelled {
				cancel()
			}
			plist := &recordingPlist{cancelOn: testCase.cancelOn, cancel: cancel}
			values, err := resolvePlist(ctx, plist, path, hits)
			if !errors.Is(err, testCase.wantError) {
				t.Errorf("error = %v, want %v", err, testCase.wantError)
			}
			if !slices.Equal(plist.calls, testCase.wantCalls) {
				t.Errorf("calls = %q, want %q", plist.calls, testCase.wantCalls)
			}
			if testCase.wantError != nil && (values.User != "" || len(values.Assignments) > 0) {
				t.Errorf("a cancelled read returned values: %+v", values)
			}
		})
	}
}

// accountPlist is a PlistReader whose UserName is the text in user.
type accountPlist struct{ user string }

// Available reports that plutil is there; it runs nothing.
func (plist accountPlist) Available() bool { return true }

// Lint accepts the file.
func (plist accountPlist) Lint(context.Context, string) bool { return true }

// ExtractString finds user at the key UserName.
func (plist accountPlist) ExtractString(_ context.Context, _, keyPath string) (string, string, bool) {
	return plist.user, "", keyPath == "UserName"
}

func TestResolvePlistKeepsAUserNameOfAtMost255Bytes(t *testing.T) {
	cases := []struct {
		name, user, want string
	}{
		{"255 bytes are kept", padded("a", "", 255), padded("a", "", 255)},
		{"256 bytes are not", padded("a", "", 256), ""},
		{"20000 bytes are not", padded("a", "", 20000), ""},
		{"a short name is kept", "svc", "svc"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			values, err := resolvePlist(context.Background(), accountPlist{test.user}, "/p.plist", nil)
			if err != nil || values.User != test.want {
				t.Errorf("User = %.40q (%d bytes), err %v; want %.40q", values.User, len(values.User),
					err, test.want)
			}
		})
	}
}
