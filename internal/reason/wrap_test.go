package reason

import (
	"errors"
	"io/fs"
	"testing"
)

func TestWrapKeepsTheTextAndTheCause(t *testing.T) {
	cause := &fs.PathError{Op: "open", Path: "/a/history.jsonl", Err: fs.ErrPermission}
	err := Wrap(Describe("history.jsonl", cause), cause)
	if got, want := err.Error(), "/a/history.jsonl: permission denied"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	var pathError *fs.PathError
	if !errors.As(err, &pathError) || pathError != cause {
		t.Errorf("errors.As found %v, want the wrapped *fs.PathError", pathError)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Error("errors.Is(err, fs.ErrPermission) = false, want true")
	}
}
