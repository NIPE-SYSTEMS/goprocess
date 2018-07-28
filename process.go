package goprocess

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"io"
)

// Events that can occur and how the process object reacts to them:
// - creation error: return error
// - Start() error: return error
// - stdin gets closed: close stdin on process
// - signals gets closed: do nothing (consequence: killing is not possible anymore)
// - process closes stdout pipe: close stdout channel
// - process closes stderr pipe: close stderr channel
// - process terminates: do nothing (pipes are closed automatically)

// NewProcess creates a new process in the background and provides a simple interface for standard I/O. It consumes and produces []byte messages that are received or will be sent to the process. The exchanged messages are split at newlines (so messages on the stdin-channel should not contain any newlines).
//
// Closing the stdin-channel will close the corresponding pipe to the process. When the process closes the stdout or stderr pipes the corresponding channels will be closed. Closing the signals-channel does nothing but makes it impossible to interact with the process via signals afterwards.
//
// To ensure that all goroutines are stopped, send a terminating signal over the signals-channel (e.g. SIGINT, SIGTERM) and close both the stdin- and signals channel.
//
// This interface does not allow to check explicitly whether the process actually exited. Nevertheless it is possible to check the closed-state of the output-channels (stdout, stderr).
//
// The following code creates a new process with all required inputs:
//
//     stdin := make(chan []byte)
//     signals := make(chan os.Signal)
//     stdout, stdin, err := NewProcess([]string{"cat"}, stdin, signals)
//     if err != nil {
//         // handle error
//     }
//
// The following code stops the process by sending a SIGINT to the process:
//
//     signals <- syscall.SIGINT
//     close(stdin)
//     close(signals)
//
// It is also possible to forward signals from the parent process to the created process:
//
//     signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
func NewProcess(args []string, stdin <-chan []byte, signals <-chan os.Signal) (<-chan []byte, <-chan []byte, error) {
	if len(args) <= 0 {
		return nil, nil, errors.New("no arguments specified")
	}
	command := exec.Command(args[0], args[1:]...)
	// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdinWriter, err := command.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stdoutScanner := bufio.NewScanner(stdoutPipe)
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	stderrScanner := bufio.NewScanner(stderrPipe)
	err = command.Start()
	if err != nil {
		return nil, nil, err
	}
	sendStdin(stdinWriter, stdin)
	stdout := recvStdout(stdoutScanner)
	stderr := recvStderr(stderrScanner)
	forwardSignals(command, signals)
	go func() {
		command.Wait()
	}()
	return stdout, stderr, nil
}

func sendStdin(stdinWriter io.WriteCloser, stdin <-chan []byte) {
	go func() {
		for msg := range stdin {
			_, err := stdinWriter.Write(append(msg, "\n"...))
			if err != nil {
				// ignore write error
				continue
			}
		}
		stdinWriter.Close()
	}()
}

func recvStdout(stdoutScanner *bufio.Scanner) <-chan []byte {
	stdout := make(chan []byte, 1024)
	go func() {
		for stdoutScanner.Scan() {
			stdout <- stdoutScanner.Bytes()
		}
		close(stdout)
	}()
	return stdout
}

func recvStderr(stderrScanner *bufio.Scanner) <-chan []byte {
	stderr := make(chan []byte, 1024)
	go func() {
		for stderrScanner.Scan() {
			stderr <- stderrScanner.Bytes()
		}
		close(stderr)
	}()
	return stderr
}

func forwardSignals(command *exec.Cmd, signals <-chan os.Signal) {
	go func() {
		for s := range signals {
			// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
			syscall.Kill(-command.Process.Pid, s.(syscall.Signal))
		}
	}()
}