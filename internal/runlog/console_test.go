package runlog

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// stalledWriter is a console that keeps its pipe open and stops reading:
// every write blocks until release is closed.
type stalledWriter struct {
	release chan struct{}
}

// Write blocks until the writer is released, then takes data.
func (writer stalledWriter) Write(data []byte) (int, error) {
	<-writer.release
	return len(data), nil
}

func TestTheConsoleWritesBothStreamsInTheOrderGiven(t *testing.T) {
	var both bytes.Buffer
	console := NewConsole(&both, &both)
	console.Err("one\n")
	console.Out("two\nthree\n")
	console.Err("four\n")
	if notShown := console.Finish(time.Minute); notShown != 0 || both.String() != "one\ntwo\nthree\nfour\n" {
		t.Errorf("wrote %q with %d lines not shown, want every line in order", both.String(), notShown)
	}
}

func TestAConsoleThatStopsReadingNeverBlocksTheSenderAndCountsWhatItDidNotShow(t *testing.T) {
	stalled := stalledWriter{release: make(chan struct{})}
	defer close(stalled.release)
	console := NewConsole(io.Discard, stalled)
	start := time.Now()
	const sent = consoleQueue * 3
	for range sent {
		console.Err("line\n")
	}
	// One line is taken by the blocked write, the queue holds consoleQueue,
	// and the rest are dropped; none of them is shown.
	notShown := console.Finish(10 * time.Millisecond)
	if notShown != sent {
		t.Errorf("%d lines not shown, want %d", notShown, sent)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("sending to a stalled console took %v", elapsed)
	}
	console.Err("after\n")
	if again := console.Finish(0); again != sent+1 {
		t.Errorf("a line sent after Finish was not counted: %d", again)
	}
}

func TestFinishWaitsForAConsoleThatStillReads(t *testing.T) {
	stalled := stalledWriter{release: make(chan struct{})}
	console := NewConsole(io.Discard, stalled)
	console.Err("slow\n")
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(stalled.release)
	}()
	if notShown := console.Finish(time.Minute); notShown != 0 {
		t.Errorf("%d lines not shown by a console that read within the wait", notShown)
	}
}

func TestFlushReturnsOnceEverythingQueuedIsWritten(t *testing.T) {
	var both bytes.Buffer
	console := NewConsole(&both, &both)
	console.Err("one\n")
	console.Out("two\n")
	if !console.Flush(time.Minute) {
		t.Fatal("Flush gave up on a console that reads")
	}
	// The writer has passed the mark, so the buffer is no longer written.
	if both.String() != "one\ntwo\n" {
		t.Errorf("Flush returned with %q written, want both lines", both.String())
	}
	if notShown := console.Finish(time.Minute); notShown != 0 {
		t.Errorf("%d lines not shown", notShown)
	}
}

func TestFlushGivesUpOnAStalledConsoleAfterItsWait(t *testing.T) {
	cases := []struct {
		name string
		sent int
	}{
		{"with room in the queue for the mark", 1},
		{"with the queue full", consoleQueue + 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stalled := stalledWriter{release: make(chan struct{})}
			defer close(stalled.release)
			console := NewConsole(io.Discard, stalled)
			for range testCase.sent {
				console.Err("line\n")
			}
			start := time.Now()
			if console.Flush(10 * time.Millisecond) {
				t.Error("Flush reported a stalled console as written")
			}
			if elapsed := time.Since(start); elapsed > 10*time.Second {
				t.Errorf("Flush on a stalled console took %v", elapsed)
			}
		})
	}
}
