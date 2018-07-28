package goprocess

import (
	"bytes"
	"os"
	"sync"
	"testing"
	"time"
)

// TestProcessCloseStdin tests if closing the stdin-channel closes the stdin-pipe. The test tests that implicitly:
//
// The test succeeds on termination of the process. The process is considered terminated if both the stdout- and stderr-channels are closed. The channels are closed when the process closes the corresponding pipes. The used process "cat" closes the pipes on exit. "cat" exits when the stdin-pipe gets closed. This means if a close of the stdin-channel leads to close of the stdout- and stderr-channels the test succeeds.
//
// The test succeeds when the process terminates within 1 second.
func TestProcessCloseStdin(t *testing.T) {
	stdin := make(chan []byte)
	signals := make(chan os.Signal)
	stdout, stderr, err := NewProcess([]string{"cat"}, stdin, signals)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
	drainloop:
		for {
			_, ok := <-stdout
			if !ok {
				break drainloop
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
	drainloop:
		for {
			_, ok := <-stderr
			if !ok {
				break drainloop
			}
		}
	}()
	done := make(chan struct{})
	go func() {
		close(stdin)
		wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("After closing the stdin-channel, the process did not terminate after 1 second.")
	}
}

// TestProcessCloseSignals tests if closing the signals-channel does not influence the started process. The test starts a new process and closes the signals-channel. The test succeeds if the process does not terminate within 1 second.
func TestProcessCloseSignals(t *testing.T) {
	stdin := make(chan []byte)
	signals := make(chan os.Signal)
	stdout, stderr, err := NewProcess([]string{"cat"}, stdin, signals)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
	drainloop:
		for {
			_, ok := <-stdout
			if !ok {
				break drainloop
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
	drainloop:
		for {
			_, ok := <-stderr
			if !ok {
				break drainloop
			}
		}
	}()
	done := make(chan struct{})
	go func() {
		close(signals)
		wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
		t.Fatal("After closing the signals-channel, the process has terminated after 1 second.")
	case <-time.After(time.Second):
	}
	close(stdin)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("After closing the stdin-channel, the process did not terminate after 1 second.")
	}
}

// TestProcessCloseStdout tests if closing the stdout-pipe closes the stdout-channel. The test succeeds when the stdout-channel is closed within 1 second and the stderr-channel within 2 seconds (i.e. the process terminates).
func TestProcessCloseStdout(t *testing.T) {
	stdin := make(chan []byte)
	signals := make(chan os.Signal)
	stdout, stderr, err := NewProcess([]string{"bash", "-c", "exec 1>&- && sleep 1"}, stdin, signals)
	if err != nil {
		t.Fatal(err)
	}
	doneStdout := make(chan struct{})
	go func() {
	drainloop:
		for {
			_, ok := <-stdout
			if !ok {
				break drainloop
			}
		}
		doneStdout <- struct{}{}
	}()
	doneStderr := make(chan struct{})
	go func() {
	drainloop:
		for {
			_, ok := <-stderr
			if !ok {
				break drainloop
			}
		}
		doneStderr <- struct{}{}
	}()
	stdoutTimer := time.NewTimer(time.Second)
	stderrTimer := time.NewTimer(2 * time.Second)
	select {
	case <-doneStdout:
	case <-stdoutTimer.C:
		t.Fatal("The process did not close stdout-channel after 1 second.")
	}
	select {
	case <-doneStderr:
	case <-stderrTimer.C:
		t.Fatal("The process did not terminate after another 1 second.")
	}
	close(stdin)
	close(signals)
}

// TestProcessCloseStderr tests if closing the stderr-pipe closes the stderr-channel. The test succeeds when the stderr-channel is closed within 1 second and the stdout-channel within 2 seconds (i.e. the process terminates).
func TestProcessCloseStderr(t *testing.T) {
	stdin := make(chan []byte)
	signals := make(chan os.Signal)
	stdout, stderr, err := NewProcess([]string{"bash", "-c", "exec 2>&- && sleep 1"}, stdin, signals)
	if err != nil {
		t.Fatal(err)
	}
	doneStdout := make(chan struct{})
	go func() {
	drainloop:
		for {
			_, ok := <-stdout
			if !ok {
				break drainloop
			}
		}
		doneStdout <- struct{}{}
	}()
	doneStderr := make(chan struct{})
	go func() {
	drainloop:
		for {
			_, ok := <-stderr
			if !ok {
				break drainloop
			}
		}
		doneStderr <- struct{}{}
	}()
	stderrTimer := time.NewTimer(time.Second)
	stdoutTimer := time.NewTimer(2 * time.Second)
	select {
	case <-doneStderr:
	case <-stderrTimer.C:
		t.Fatal("The process did not close stderr-channel after 1 second.")
	}
	select {
	case <-doneStdout:
	case <-stdoutTimer.C:
		t.Fatal("The process did not terminate after another 1 second.")
	}
	close(stdin)
	close(signals)
}

// TestProcessTerminate tests if terminating the process closes the stdout- and stderr-channels. The test succeeds when both channels are being closed within 1 second and the consumed messages match the expected messages.
func TestProcessTerminate(t *testing.T) {
	stdin := make(chan []byte)
	signals := make(chan os.Signal)
	stdout, stderr, err := NewProcess([]string{"echo", "Test"}, stdin, signals)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stdoutMessages := make([][]byte, 0)
	stderrMessages := make([][]byte, 0)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range stdout {
			stdoutMessages = append(stdoutMessages, msg)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range stderr {
			stderrMessages = append(stderrMessages, msg)
		}
	}()
	done := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("The process did not terminate after 1 second.")
	}
	close(stdin)
	close(signals)
	if len(stdoutMessages) != 1 {
		t.Fatalf("Got %d messages, expected %d messages.", len(stdoutMessages), 1)
	}
	if len(stderrMessages) != 0 {
		t.Fatalf("Got %d messages, expected %d messages.", len(stdoutMessages), 0)
	}
	if bytes.Compare(stdoutMessages[0], []byte("Test")) != 0 {
		t.Fatalf("Process send %v to stdout, expected %v.", stdoutMessages[0], []byte("Test"))
	}
}
