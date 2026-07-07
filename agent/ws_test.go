package main

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

// mockConn implements net.Conn for testing WebSocket helpers.
type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
}

func newMockConn() *mockConn {
	return &mockConn{
		readBuf:  &bytes.Buffer{},
		writeBuf: &bytes.Buffer{},
	}
}

func (c *mockConn) Read(b []byte) (int, error) {
	if c.closed {
		return 0, io.EOF
	}
	return c.readBuf.Read(b)
}

func (c *mockConn) Write(b []byte) (int, error) {
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	return c.writeBuf.Write(b)
}

func (c *mockConn) Close() error {
	c.closed = true
	return nil
}

func (c *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (c *mockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (c *mockConn) SetDeadline(t time.Time) error      { return nil }
func (c *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// ---------------------------------------------------------------------------
// wsWriteFrame
// ---------------------------------------------------------------------------

func TestWsWriteFrame_SmallPayload(t *testing.T) {
	conn := newMockConn()
	data := []byte("hello")

	err := wsWriteFrame(conn, data)
	if err != nil {
		t.Fatalf("wsWriteFrame: %v", err)
	}

	frame := conn.writeBuf.Bytes()
	if len(frame) < 2 {
		t.Fatalf("frame too short: %d bytes", len(frame))
	}
	// First byte: 0x81 (FIN + text opcode)
	if frame[0] != 0x81 {
		t.Errorf("first byte = 0x%02x, want 0x81", frame[0])
	}
	// Second byte: payload length (5)
	if frame[1] != 5 {
		t.Errorf("length byte = %d, want 5", frame[1])
	}
	// Payload
	if string(frame[2:]) != "hello" {
		t.Errorf("payload = %q, want %q", string(frame[2:]), "hello")
	}
}

func TestWsWriteFrame_MediumPayload(t *testing.T) {
	conn := newMockConn()
	// 200 bytes — triggers the 126 extended length path
	data := make([]byte, 200)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := wsWriteFrame(conn, data)
	if err != nil {
		t.Fatalf("wsWriteFrame: %v", err)
	}

	frame := conn.writeBuf.Bytes()
	if frame[0] != 0x81 {
		t.Errorf("first byte = 0x%02x, want 0x81", frame[0])
	}
	if frame[1] != 126 {
		t.Errorf("length indicator = %d, want 126", frame[1])
	}
	// Next 2 bytes are big-endian length
	extLen := int(frame[2])<<8 | int(frame[3])
	if extLen != 200 {
		t.Errorf("extended length = %d, want 200", extLen)
	}
	if len(frame) != 4+200 {
		t.Errorf("total frame length = %d, want %d", len(frame), 4+200)
	}
}

func TestWsWriteFrame_LargePayload(t *testing.T) {
	conn := newMockConn()
	// 70000 bytes — triggers the 127 extended length path
	data := make([]byte, 70000)

	err := wsWriteFrame(conn, data)
	if err != nil {
		t.Fatalf("wsWriteFrame: %v", err)
	}

	frame := conn.writeBuf.Bytes()
	if frame[0] != 0x81 {
		t.Errorf("first byte = 0x%02x, want 0x81", frame[0])
	}
	if frame[1] != 127 {
		t.Errorf("length indicator = %d, want 127", frame[1])
	}
	// Total frame: 2 + 8 + 70000
	if len(frame) != 10+70000 {
		t.Errorf("total frame length = %d, want %d", len(frame), 10+70000)
	}
}

func TestWsWriteFrame_EmptyPayload(t *testing.T) {
	conn := newMockConn()
	err := wsWriteFrame(conn, []byte{})
	if err != nil {
		t.Fatalf("wsWriteFrame: %v", err)
	}
	frame := conn.writeBuf.Bytes()
	if len(frame) != 2 {
		t.Errorf("frame length = %d, want 2", len(frame))
	}
	if frame[1] != 0 {
		t.Errorf("length byte = %d, want 0", frame[1])
	}
}

func TestWsWriteFrame_ClosedConn(t *testing.T) {
	conn := newMockConn()
	conn.closed = true
	err := wsWriteFrame(conn, []byte("data"))
	if err == nil {
		t.Error("expected error writing to closed connection")
	}
}

// ---------------------------------------------------------------------------
// wsWritePing
// ---------------------------------------------------------------------------

func TestWsWritePing_Success(t *testing.T) {
	conn := newMockConn()
	err := wsWritePing(conn)
	if err != nil {
		t.Fatalf("wsWritePing: %v", err)
	}

	frame := conn.writeBuf.Bytes()
	if len(frame) != 2 {
		t.Fatalf("ping frame length = %d, want 2", len(frame))
	}
	if frame[0] != 0x89 {
		t.Errorf("ping opcode = 0x%02x, want 0x89", frame[0])
	}
	if frame[1] != 0x00 {
		t.Errorf("ping length = 0x%02x, want 0x00", frame[1])
	}
}

func TestWsWritePing_ClosedConn(t *testing.T) {
	conn := newMockConn()
	conn.closed = true
	err := wsWritePing(conn)
	if err == nil {
		t.Error("expected error writing ping to closed connection")
	}
}

// ---------------------------------------------------------------------------
// wsReadDiscard
// ---------------------------------------------------------------------------

func TestWsReadDiscard_ReadsAndReturns(t *testing.T) {
	conn := newMockConn()
	conn.readBuf.Write([]byte("some ws frames to discard"))

	done := make(chan struct{})
	go func() {
		wsReadDiscard(conn)
		close(done)
	}()

	select {
	case <-done:
		// OK — returned after reading all data (EOF)
	case <-time.After(2 * time.Second):
		t.Fatal("wsReadDiscard did not return after EOF")
	}
}

func TestWsReadDiscard_ClosedConn(t *testing.T) {
	conn := newMockConn()
	conn.closed = true

	done := make(chan struct{})
	go func() {
		wsReadDiscard(conn)
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("wsReadDiscard did not return on closed connection")
	}
}
