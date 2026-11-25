package runlog

import (
	"io"
	"strings"
	"sync/atomic"
	"time"
)

// consoleQueue is how many console texts (a log line, or the summary) may
// wait to be written. A console that is slow for a moment (a remote shell
// relaying in bursts) loses nothing, while one that has stopped reading
// holds at most this many texts in memory; beyond it a text is dropped from
// the console, never from the log file.
const consoleQueue = 4096

// Console writes the run's standard output and standard error text in the
// order it is given, from one goroutine of its own, so a reader that keeps
// its pipe open but stops reading blocks that goroutine and never the run.
// A write's error is dropped: the log file is the record.
type Console struct {
	stdout, stderr io.Writer
	queue          chan consoleText
	done           chan struct{}
	// queued, notQueued and finished belong to the goroutine that sends;
	// shown is counted by the writer.
	queued, notQueued int64
	finished          bool
	shown             atomic.Int64
}

// consoleText is one piece of text and the stream it goes to, or, with
// flushed set, a mark the writer closes flushed at (Flush).
type consoleText struct {
	out     io.Writer
	text    string
	lines   int64
	flushed chan struct{}
}

// NewConsole starts the writer of stdout and stderr and returns its Console.
// Text is sent from one goroutine at a time, the run's own.
func NewConsole(stdout, stderr io.Writer) *Console {
	console := &Console{stdout: stdout, stderr: stderr, queue: make(chan consoleText, consoleQueue),
		done: make(chan struct{})}
	go console.write()
	return console
}

// write writes each queued text until Finish closes the queue.
func (console *Console) write() {
	defer close(console.done)
	for each := range console.queue {
		if each.flushed != nil {
			close(each.flushed)
			continue
		}
		_, _ = io.WriteString(each.out, each.text)
		console.shown.Add(each.lines)
	}
}

// Out queues text for standard output.
func (console *Console) Out(text string) { console.send(console.stdout, text) }

// Err queues text for standard error.
func (console *Console) Err(text string) { console.send(console.stderr, text) }

// send queues text for out, or counts it as not shown when the queue is
// full or the console has finished.
func (console *Console) send(out io.Writer, text string) {
	lines := int64(strings.Count(text, "\n"))
	if console.finished {
		console.notQueued += lines
		return
	}
	select {
	case console.queue <- consoleText{out: out, text: text, lines: lines}:
		console.queued += lines
	default:
		console.notQueued += lines
	}
}

// Flush waits at most wait for the writer to write every text queued so
// far and reports whether it did, so that output written directly next can
// follow them. A console that has not taken them by then is taken as
// stopped: the caller goes on, and the texts may still appear later.
func (console *Console) Flush(wait time.Duration) bool {
	// A finished console's queue is closed and takes no mark.
	if console.finished {
		return false
	}
	flushed := make(chan struct{})
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case console.queue <- consoleText{flushed: flushed}:
	case <-timer.C:
		return false
	}
	select {
	case <-flushed:
		return true
	case <-timer.C:
		return false
	}
}

// Finish stops taking text, waits at most wait for what is queued to be
// written, and returns how many lines were not shown: dropped from a full
// queue or still unwritten when the wait ended. A writer still blocked then
// is left behind, since a write blocked on a pipe cannot be interrupted;
// the process's exit ends it.
func (console *Console) Finish(wait time.Duration) int64 {
	if !console.finished {
		console.finished = true
		close(console.queue)
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-console.done:
	case <-timer.C:
	}
	return console.notQueued + console.queued - console.shown.Load()
}
