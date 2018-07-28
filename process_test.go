package interfaces

import (
	"strconv"
	"testing"
	"time"
)

func TestProcessOutputs(t *testing.T) {
	messagesIn := make(chan *Message)
	messagesOut, err := StartProcess([]string{"bash", "-c", "printf 'foo\\0bar\\n' && printf 'foo\\0bar2\\n'"}, messagesIn)
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}
	messagesSlice := make([]*Message, 0)
	for message := range messagesOut {
		messagesSlice = append(messagesSlice, message)
	}
	if len(messagesSlice) != 2 {
		t.Fatal("missing messages: expected 2 messages, got", len(messagesSlice), "messages")
	}
	if !messagesSlice[0].EqualValues("foo", []byte("bar")) {
		t.Fatalf("message 1 not matching: got %#v, expected %#v", messagesSlice[0], NewMessage("foo", []byte("bar")))
	}
	if !messagesSlice[1].EqualValues("foo", []byte("bar2")) {
		t.Fatalf("message 2 not matching: got %#v, expected %#v", messagesSlice[1], NewMessage("foo", []byte("bar2")))
	}
}

func TestProcessCancel(t *testing.T) {
	messagesIn := make(chan *Message)
	messagesOut, err := StartProcess([]string{"bash", "-c", "printf 'foo\\0bar\\n' && sleep 0.2 && printf 'foo\\0bar2\\n'"}, messagesIn)
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}
	go func() {
		// cancel after 100 milliseconds
		time.Sleep(100 * time.Millisecond)
		close(messagesIn)
	}()
	messagesSlice := make([]*Message, 0)
	for message := range messagesOut {
		messagesSlice = append(messagesSlice, message)
	}
	// now there should be only one message because the second message should
	// come after 100 more milliseconds
	if len(messagesSlice) != 1 {
		t.Fatal("missing messages: expected 1 message, got", len(messagesSlice), "messages")
	}
	if !messagesSlice[0].EqualValues("foo", []byte("bar")) {
		t.Fatalf("message 1 not matching: got %#v, expected %#v", messagesSlice[0], NewMessage("foo", []byte("bar")))
	}
}

func TestProcessInputOutput(t *testing.T) {
	messagesIn := make(chan *Message)
	messagesOut, err := StartProcess([]string{"cat"}, messagesIn)
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}
	go func() {
		// send 10 messages
		for i := 0; i < 10; i++ {
			messagesIn <- NewMessage("foo", []byte(strconv.Itoa(i)))
		}
		// grace period for process to output the messages again
		time.Sleep(100 * time.Millisecond)
		// cancel after 10 messages
		close(messagesIn)
	}()
	messagesSlice := make([]*Message, 0)
	for message := range messagesOut {
		messagesSlice = append(messagesSlice, message)
	}
	if len(messagesSlice) != 10 {
		t.Fatal("missing messages: expected 10 message, got", len(messagesSlice), "messages")
	}
	for i := 0; i < 10; i++ {
		if !messagesSlice[i].EqualValues("foo", []byte(strconv.Itoa(i))) {
			t.Fatalf("message %d not matching: got %#v, expected %#v", i, messagesSlice[i], NewMessage("foo", []byte(strconv.Itoa(i))))
		}
	}
}
