package redis

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// RESP3 wire protocol encoder/decoder.
// Reference: https://redis.io/docs/reference/protocol-spec/

// WriteCommand writes a RESP3 array command (e.g. ["SET", "key", "value"]).
func WriteCommand(writer io.Writer, args ...string) error {
	var builder strings.Builder
	builder.WriteString("*")
	builder.WriteString(strconv.Itoa(len(args)))
	builder.WriteString("\r\n")
	for _, arg := range args {
		builder.WriteString("$")
		builder.WriteString(strconv.Itoa(len(arg)))
		builder.WriteString("\r\n")
		builder.WriteString(arg)
		builder.WriteString("\r\n")
	}
	_, err := io.WriteString(writer, builder.String())
	return err
}

// ReadResponse reads a single RESP3 response. Returns the parsed value or
// an error. Supported types: simple string (+), error (-), integer (:),
// bulk string ($), array (*), null (_).
func ReadResponse(reader *bufio.Reader) (interface{}, error) {
	line, err := readLine(reader)
	if err != nil {
		return nil, err
	}
	if line == "" {
		return nil, fmt.Errorf("redis: empty response line")
	}

	prefix := line[0]
	payload := line[1:]

	switch prefix {
	case '+':
		return payload, nil

	case '-':
		return nil, &Error{Message: payload}

	case ':':
		n, parseErr := strconv.ParseInt(payload, 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("redis: invalid integer %q: %w", payload, parseErr)
		}
		return n, nil

	case '$':
		return readBulkString(reader, payload)

	case '*':
		return readArray(reader, payload)

	case '_':
		return nil, nil

	default:
		return nil, fmt.Errorf("redis: unknown response type %q", string(prefix))
	}
}

// Error is returned when the Redis server responds with an error.
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return "redis: " + e.Message
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("redis: read line: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func readBulkString(reader *bufio.Reader, lengthStr string) (interface{}, error) {
	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return nil, fmt.Errorf("redis: invalid bulk length %q: %w", lengthStr, err)
	}
	if length == -1 {
		return nil, nil
	}
	if length < 0 {
		return nil, fmt.Errorf("redis: negative bulk length %d", length)
	}
	buf := make([]byte, length+2) // +2 for trailing \r\n
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("redis: read bulk string: %w", err)
	}
	return string(buf[:length]), nil
}

func readArray(reader *bufio.Reader, countStr string) (interface{}, error) {
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return nil, fmt.Errorf("redis: invalid array count %q: %w", countStr, err)
	}
	if count == -1 {
		return nil, nil
	}
	if count < 0 {
		return nil, fmt.Errorf("redis: negative array count %d", count)
	}
	result := make([]interface{}, count)
	for i := range count {
		elem, readErr := ReadResponse(reader)
		if readErr != nil {
			return nil, fmt.Errorf("redis: read array element %d: %w", i, readErr)
		}
		result[i] = elem
	}
	return result, nil
}
