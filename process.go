package interfaces

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
)

// close(messagesIn) kills the process and therefore ends it
// when the process ends (by itself or by a kill) it closes the messageOut-channel
func StartProcess(args []string, messagesIn chan *Message) (chan *Message, error) {
	if len(args) == 0 {
		return nil, errors.New("no arguments specified")
	}
	command := exec.Command(args[0], args[1:]...)
	/*stdin, stdout, stderr,*/ stdinWriter, stdoutScanner, stderrScanner, err := getReaders(command)
	if err != nil {
		return nil, err
	}
	// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// doneStdin informs about the closed messagesIn-channel
	doneStdin := sendStdin(stdinWriter, messagesIn)
	// doneStdout informs about the closed stdout-pipe from the process (usually triggers with doneProcess)
	messagesOut, doneStdout := recvStdout(stdoutScanner)
	pipeStderr(stderrScanner)
	// doneProcess informs about the terminated process (usually triggers with doneStdout)
	doneProcess := runProcess(command)
	go func() {
		select {
		case <-doneStdin:
			// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
			syscall.Kill(-command.Process.Pid, syscall.SIGINT)
		case <-doneStdout:
		case <-doneProcess:
		}
	}()
	return messagesOut, nil
}

func getReaders(command *exec.Cmd) ( /*io.WriteCloser, io.ReadCloser, io.ReadCloser,*/ io.WriteCloser, *bufio.Scanner, *bufio.Scanner, error) {
	stdin, err := command.StdinPipe()
	if err != nil {
		return /*nil, nil, nil,*/ nil, nil, nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return /*nil, nil, nil,*/ nil, nil, nil, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return /*nil, nil, nil,*/ nil, nil, nil, err
	}
	return /*stdin, stdout, stderr,*/ stdin, bufio.NewScanner(stdout), bufio.NewScanner(stderr), nil
}

func sendStdin(stdinWriter io.WriteCloser, messagesIn chan *Message) chan struct{} {
	done := make(chan struct{}, 1) // will not block since done has capacity = 1
	go func() {
		for message := range messagesIn {
			_, err := stdinWriter.Write(append(message.Bytes(), "\n"...))
			if err != nil {
				log.Println("failed to write message:", err)
				continue
			}
		}
		done <- struct{}{}
	}()
	return done
}

func recvStdout(stdoutScanner *bufio.Scanner) (chan *Message, chan struct{}) {
	messagesOut := make(chan *Message, 1024)
	done := make(chan struct{}, 1) // will not block since done has capacity = 1
	go func() {
		for stdoutScanner.Scan() {
			message, err := NewMessageFromData(stdoutScanner.Bytes())
			if err != nil {
				log.Println("failed to parse message:", err)
				continue
			}
			messagesOut <- message
		}
		close(messagesOut)
		done <- struct{}{}
	}()
	return messagesOut, done
}

// ignore the closing of stderr from the process (so there is no done channel)
func pipeStderr(stderrScanner *bufio.Scanner) {
	go func() {
		for stderrScanner.Scan() {
			fmt.Fprintln(os.Stderr, stderrScanner.Text())
		}
	}()
}

func runProcess(command *exec.Cmd) chan struct{} {
	done := make(chan struct{}, 1) // will not block since done has capacity = 1
	go func() {
		command.Run()
		done <- struct{}{}
	}()
	return done
}
