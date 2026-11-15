package fsmeta

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// refusingStatx returns a statx that fails with refusal and counts its calls.
func refusingStatx(refusal error, calls *int) statxCall {
	return func(int, string, int, int, *unix.Statx_t) error {
		*calls++
		return refusal
	}
}

func TestLstatFallsBackToLstatWhenStatxIsRefused(t *testing.T) {
	directory := t.TempDir()
	want, err := Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	want.Btime = 0
	for _, testCase := range []struct {
		name    string
		refusal error
	}{
		{"a kernel without statx (ENOSYS) is answered by lstat", unix.ENOSYS},
		{"a filter refusing statx (EPERM) is answered by lstat", unix.EPERM},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			statxCalls, lstatCalls := 0, 0
			lstat := func(path string, stat *unix.Stat_t) error {
				lstatCalls++
				return unix.Lstat(path, stat)
			}
			got, err := lstatWith(directory, refusingStatx(testCase.refusal, &statxCalls), lstat)
			if err != nil {
				t.Fatal(err)
			}
			if statxCalls != 1 || lstatCalls != 1 {
				t.Errorf("statx calls = %d, lstat calls = %d; want 1 and 1", statxCalls, lstatCalls)
			}
			if got.Kind != Dir || got.Btime != 0 {
				t.Errorf("kind = %v, btime = %d; want a directory with btime 0", got.Kind, got.Btime)
			}
			if got != want {
				t.Errorf("lstat gave %+v\nstatx gave %+v (btime cleared)", got, want)
			}
		})
	}
}

func TestLstatFallbackReportsTheLstatError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	calls := 0
	_, err := lstatWith(missing, refusingStatx(unix.EPERM, &calls), unix.Lstat)
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "lstat" || pathErr.Path != missing {
		t.Fatalf("error = %v, want an lstat *fs.PathError for %s", err, missing)
	}
	if !errors.Is(err, unix.ENOENT) || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want one wrapping ENOENT", err)
	}
}

func TestLstatKeepsAnyOtherStatxError(t *testing.T) {
	calls, lstatCalls := 0, 0
	lstat := func(string, *unix.Stat_t) error {
		lstatCalls++
		return nil
	}
	_, err := lstatWith(t.TempDir(), refusingStatx(unix.EACCES, &calls), lstat)
	if !errors.Is(err, unix.EACCES) || lstatCalls != 0 {
		t.Errorf("error = %v, lstat calls = %d; want EACCES and no lstat", err, lstatCalls)
	}
}
