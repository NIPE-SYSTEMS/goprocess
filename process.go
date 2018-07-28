package goprocess

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
)

// close(messagesIn) kills the process and therefore ends it
// when the process ends (by itself or by a kill) it closes the messageOut-channel
func NewProcess(args []string, stdin chan []byte, signals chan os.Signal) (chan []byte, chan []byte, error) {
	if len(args) <= 0 {
		return nil, nil, errors.New("no arguments specified")
	}
	command := exec.Command(args[0], args[1:]...)
	stdinWriter, stdoutScanner, stderrScanner, err := getReaders(command)
	if err != nil {
		return nil, nil, err
	}
	// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// doneStdin informs about the closed messagesIn-channel
	doneStdin := sendStdin(stdinWriter, stdin)
	// doneStdout/doneStderr informs about the closed stdout-/stderr-pipe from the process (usually triggers with doneProcess)
	stdout, doneStdout := recvStdout(stdoutScanner)
	stderr, doneStderr := recvStderr(stderrScanner)
	// doneProcess informs about the terminated process (usually triggers with doneStdout)
	doneProcess := runProcess(command)
	forwardSignals(command, signals)
	go func() {
		select {
		case <-doneStdin:
			// TODO: ignore stdin closing
			// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
			syscall.Kill(-command.Process.Pid, syscall.SIGINT)
		case <-doneStdout:
		case <-doneStderr:
		case <-doneProcess:
		}
	}()
	return stdout, stderr, nil
}

func getReaders(command *exec.Cmd) (io.WriteCloser, *bufio.Scanner, *bufio.Scanner, error) {
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	return stdin, bufio.NewScanner(stdout), bufio.NewScanner(stderr), nil
}

func sendStdin(stdinWriter io.WriteCloser, stdin chan []byte) chan struct{} {
	done := make(chan struct{}, 1) // will not block since done has capacity = 1
	go func() {
		for msg := range stdin {
			_, err := stdinWriter.Write(append(msg, "\n"...))
			if err != nil {
				//log.Println("failed to write msg:", err)
				// ignore write error
				continue
			}
			// TODO: flush
		}
		done <- struct{}{}
	}()
	return done
}

func recvStdout(stdoutScanner *bufio.Scanner) (chan []byte, chan struct{}) {
	stdout := make(chan []byte, 1024)
	done := make(chan struct{}, 1) // will not block since done has capacity = 1
	go func() {
		for stdoutScanner.Scan() {
			stdout <- stdoutScanner.Bytes()
		}
		close(stdout)
		done <- struct{}{}
	}()
	return stdout, done
}

func recvStderr(stderrScanner *bufio.Scanner) (chan []byte, chan struct{}) {
	stderr := make(chan []byte, 1024)
	done := make(chan struct{}, 1) // will not block since done has capacity = 1
	go func() {
		for stderrScanner.Scan() {
			stderr <- stderrScanner.Bytes()
		}
		close(stderr)
		done <- struct{}{}
	}()
	return stderr, done
}

func runProcess(command *exec.Cmd) chan struct{} {
	done := make(chan struct{}, 1) // will not block since done has capacity = 1
	go func() {
		command.Run()
		done <- struct{}{}
	}()
	return done
}

func forwardSignals(command *exec.Cmd, signals chan os.Signal) {
	go func() {
		for s := range signals {
			// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
			syscall.Kill(-command.Process.Pid, s.(syscall.Signal))
		}
	}()
}