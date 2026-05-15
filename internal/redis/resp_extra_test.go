package redis

import (
	"bufio"
	"strings"
	"testing"
)

// TestReadResponseTruncatedBulkString tests a truncated bulk string read.
func TestReadResponseTruncatedBulkString(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("$10\r\nshort"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Error("expected error for truncated bulk string")
	}
}

// TestReadLineError tests that readLine returns error on empty reader.
func TestReadLineError(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Error("expected error for empty reader")
	}
}

// TestReadResponseArrayWithError tests reading an array that contains an error element.
func TestReadResponseArrayWithErrorElement(t *testing.T) {
	// Array with 1 element that is truncated.
	reader := bufio.NewReader(strings.NewReader("*1\r\n$10\r\nshort"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Error("expected error for array with truncated element")
	}
}
