package redis

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWriteCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			"ping",
			[]string{"PING"},
			"*1\r\n$4\r\nPING\r\n",
		},
		{
			"set",
			[]string{"SET", "key", "value"},
			"*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n",
		},
		{
			"get",
			[]string{"GET", "key"},
			"*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n",
		},
		{
			"empty args",
			[]string{},
			"*0\r\n",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteCommand(&buf, testCase.args...)
			if err != nil {
				t.Fatalf("WriteCommand error: %v", err)
			}
			got := buf.String()
			if got != testCase.want {
				t.Errorf("WriteCommand(%v)\ngot  %q\nwant %q", testCase.args, got, testCase.want)
			}
		})
	}
}

func TestReadResponseSimpleString(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("+OK\r\n"))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if s != "OK" {
		t.Errorf("got %q, want %q", s, "OK")
	}
}

func TestReadResponseError(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("-ERR unknown command\r\n"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var redisErr *RedisError
	if !errors.As(err, &redisErr) {
		t.Fatalf("expected *RedisError, got %T: %v", err, err)
	}
	if redisErr.Message != "ERR unknown command" {
		t.Errorf("error message: got %q, want %q", redisErr.Message, "ERR unknown command")
	}
}

func TestReadResponseInteger(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(":42\r\n"))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	n, ok := result.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", result)
	}
	if n != 42 {
		t.Errorf("got %d, want 42", n)
	}
}

func TestReadResponseNegativeInteger(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(":-1\r\n"))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	n, ok := result.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", result)
	}
	if n != -1 {
		t.Errorf("got %d, want -1", n)
	}
}

func TestReadResponseBulkString(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("$5\r\nhello\r\n"))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if s != "hello" {
		t.Errorf("got %q, want %q", s, "hello")
	}
}

func TestReadResponseBulkStringNull(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("$-1\r\n"))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestReadResponseArray(t *testing.T) {
	input := "*3\r\n$3\r\nfoo\r\n$3\r\nbar\r\n:42\r\n"
	reader := bufio.NewReader(strings.NewReader(input))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(arr) != 3 {
		t.Fatalf("array length: got %d, want 3", len(arr))
	}
	if arr[0].(string) != "foo" {
		t.Errorf("arr[0]: got %q, want %q", arr[0], "foo")
	}
	if arr[1].(string) != "bar" {
		t.Errorf("arr[1]: got %q, want %q", arr[1], "bar")
	}
	if arr[2].(int64) != 42 {
		t.Errorf("arr[2]: got %d, want 42", arr[2])
	}
}

func TestReadResponseNullArray(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("*-1\r\n"))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestReadResponseEmptyArray(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("*0\r\n"))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(arr) != 0 {
		t.Errorf("array length: got %d, want 0", len(arr))
	}
}

func TestReadResponseNull(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("_\r\n"))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestReadResponseUnknownType(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("~unknown\r\n"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
}

func TestReadResponseEmptyLine(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\r\n"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Fatal("expected error for empty line, got nil")
	}
}

func TestReadResponseInvalidInteger(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(":notanumber\r\n"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Fatal("expected error for invalid integer, got nil")
	}
}

func TestReadResponseInvalidBulkLength(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("$notanumber\r\n"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Fatal("expected error for invalid bulk length, got nil")
	}
}

func TestReadResponseNegativeBulkLength(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("$-2\r\n"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Fatal("expected error for negative bulk length, got nil")
	}
}

func TestReadResponseInvalidArrayCount(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("*notanumber\r\n"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Fatal("expected error for invalid array count, got nil")
	}
}

func TestReadResponseNegativeArrayCount(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("*-2\r\n"))
	_, err := ReadResponse(reader)
	if err == nil {
		t.Fatal("expected error for negative array count, got nil")
	}
}

func TestRedisErrorString(t *testing.T) {
	err := &RedisError{Message: "test error"}
	want := "redis: test error"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestReadResponseEmptyBulkString(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("$0\r\n\r\n"))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if s != "" {
		t.Errorf("got %q, want empty string", s)
	}
}

func TestReadResponseNestedArray(t *testing.T) {
	input := "*2\r\n*2\r\n$1\r\na\r\n$1\r\nb\r\n*1\r\n:99\r\n"
	reader := bufio.NewReader(strings.NewReader(input))
	result, err := ReadResponse(reader)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(arr) != 2 {
		t.Fatalf("outer array length: got %d, want 2", len(arr))
	}
	inner, ok := arr[0].([]interface{})
	if !ok {
		t.Fatalf("arr[0]: expected []interface{}, got %T", arr[0])
	}
	if len(inner) != 2 {
		t.Fatalf("inner array length: got %d, want 2", len(inner))
	}
	if inner[0].(string) != "a" {
		t.Errorf("inner[0]: got %q, want %q", inner[0], "a")
	}
}
