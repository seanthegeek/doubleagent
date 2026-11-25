package collect

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// stalledConsole is a console that keeps its pipe open and stops reading:
// every write blocks until release is closed.
type stalledConsole struct {
	release chan struct{}
}

// Write blocks until the console is released, then takes data.
func (console stalledConsole) Write(data []byte) (int, error) {
	<-console.release
	return len(data), nil
}

// stallLimit bounds a run over a stalled console, so a run that blocks on
// it fails the test instead of hanging it.
const stallLimit = time.Minute

func TestARunWhoseConsoleStopsReadingFinishesAndLogsWhatItDidNotShow(t *testing.T) {
	cases := []struct {
		name string
		// stallStdout and stallStderr choose which stream stops reading.
		stallStdout, stallStderr bool
		wantNotShown             string
	}{
		{"a standard error that stops reading", false, true, `console: [1-9][0-9]* lines not shown`},
		{"a standard output that stops reading", true, false, `console: [1-9][0-9]* lines not shown`},
		{"both streams stopped", true, true, `console: [1-9][0-9]* lines not shown`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name+" leaves the run finished and its log whole", func(t *testing.T) {
			stalled := stalledConsole{release: make(chan struct{})}
			defer close(stalled.release)
			stdout, stderr := io.Writer(&bytes.Buffer{}), io.Writer(&bytes.Buffer{})
			if testCase.stallStdout {
				stdout = stalled
			}
			if testCase.stallStderr {
				stderr = stalled
			}
			outDir := longTempDir(t)
			options := testOptions(fixtureImage(t), outDir)
			options.Quiet = false
			host := testHost(t)
			host.consoleLimit = 10 * time.Millisecond
			if code := runBounded(t, options, host, stdout, stderr); code != ExitOK {
				t.Fatalf("exit code %d, want %d", code, ExitOK)
			}
			log := runOutput(t, outDir, ".log")
			if !strings.Contains(log, " done: ") {
				t.Errorf("the log has no done line:\n%s", log)
			}
			if last := lastLogLine(t, outDir); !regexp.MustCompile(` ` + testCase.wantNotShown + `$`).MatchString(last) {
				t.Errorf("the log ends %q, want %q", last, testCase.wantNotShown)
			}
			if matches, _ := filepath.Glob(filepath.Join(outDir, "*.tar.gz")); len(matches) != 1 {
				t.Errorf("the run wrote the archives %v, want one", matches)
			}
		})
	}
}

func TestARunThatCannotStartEndsWhenItsConsoleStopsReading(t *testing.T) {
	stalled := stalledConsole{release: make(chan struct{})}
	defer close(stalled.release)
	options := testOptions(filepath.Join(t.TempDir(), "missing"), t.TempDir())
	host := testHost(t)
	host.consoleLimit = 10 * time.Millisecond
	if code := runBounded(t, options, host, &bytes.Buffer{}, stalled); code != ExitFatal {
		t.Errorf("exit code %d, want %d", code, ExitFatal)
	}
}

func TestAConsoleThatReadsGetsEveryLineAndTheLogNoCount(t *testing.T) {
	var stdout, stderr bytes.Buffer
	outDir := longTempDir(t)
	options := testOptions(fixtureImage(t), outDir)
	options.Quiet = false
	if code := runBounded(t, options, testHost(t), &stdout, &stderr); code != ExitOK {
		t.Fatalf("exit code %d, want %d", code, ExitOK)
	}
	if !strings.HasSuffix(stderr.String(), "Archiving\ndone\n") || !strings.HasPrefix(stdout.String(), "archive:") {
		t.Errorf("stderr %q and stdout %q, want every line", stderr.String(), stdout.String())
	}
	if log := runOutput(t, outDir, ".log"); strings.Contains(log, "console:") {
		t.Errorf("the log counts lines not shown although the console read them:\n%s", log)
	}
}

// sharedConsole is standard output and standard error sent to one place
// (`2>&1`): each standard error write waits until release is closed, and
// each write's stream is recorded in the order the writes end.
type sharedConsole struct {
	mutex   sync.Mutex
	streams []string
	release chan struct{}
	once    sync.Once
}

// releaseAll lets every held standard error write end.
func (console *sharedConsole) releaseAll() { console.once.Do(func() { close(console.release) }) }

// sharedStream is one stream of a sharedConsole.
type sharedStream struct {
	console *sharedConsole
	name    string
}

// Write records the stream's write once it may end: standard error after
// the release, standard output at once, releasing standard error too, so a
// run that does not wait for its console does not hang the test.
func (stream sharedStream) Write(data []byte) (int, error) {
	if stream.name == "stderr" {
		<-stream.console.release
	} else {
		stream.console.releaseAll()
	}
	stream.console.mutex.Lock()
	defer stream.console.mutex.Unlock()
	stream.console.streams = append(stream.console.streams, stream.name)
	return len(data), nil
}

func TestTheInventoryWaitsForTheProgressLinesQueuedBeforeIt(t *testing.T) {
	console := &sharedConsole{release: make(chan struct{})}
	defer console.releaseAll()
	options := testOptions(fixtureImage(t), "")
	options.Quiet, options.Inventory = false, true
	host := testHost(t)
	host.consoleLimit = stallLimit
	// The progress lines are held until the run says it waits for them.
	host.reached = func(point string) {
		if point == consoleFlushing {
			console.releaseAll()
		}
	}
	stdout, stderr := sharedStream{console, "stdout"}, sharedStream{console, "stderr"}
	if code := runBounded(t, options, host, stdout, stderr); code != ExitOK {
		t.Fatalf("exit code %d, want %d", code, ExitOK)
	}
	order := strings.Join(console.streams, " ")
	first := strings.Index(order, "stdout")
	if first < 0 || strings.Contains(order[first:], "stderr") || !strings.Contains(order, "stderr") {
		t.Errorf("writes ended in the order %s, want every progress line before the inventory", order)
	}
}

func TestAnInventoryWhoseConsoleStopsReadingIsStillWritten(t *testing.T) {
	stalled := stalledConsole{release: make(chan struct{})}
	defer close(stalled.release)
	options := testOptions(fixtureImage(t), "")
	options.Quiet, options.Inventory = false, true
	host := testHost(t)
	host.consoleLimit = 10 * time.Millisecond
	var stdout bytes.Buffer
	if code := runBounded(t, options, host, &stdout, stalled); code != ExitOK {
		t.Fatalf("exit code %d, want %d", code, ExitOK)
	}
	if !strings.HasPrefix(stdout.String(), `{"type":"host"`) || !strings.Contains(stdout.String(), `"user":"alice"`) {
		t.Errorf("stdout %q, want the whole inventory", stdout.String())
	}
}

// runBounded runs a collection and returns its exit code, failing the test
// when the run has not returned within stallLimit.
func runBounded(t *testing.T, options Options, host environment, stdout, stderr io.Writer) int {
	t.Helper()
	codes := make(chan int, 1)
	go func() { codes <- runCollection(context.Background(), options, host, stdout, stderr) }()
	select {
	case code := <-codes:
		return code
	case <-time.After(stallLimit):
		t.Fatalf("the run had not returned after %v: it is blocked on its console", stallLimit)
	}
	return 0
}
