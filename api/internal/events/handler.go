package events

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/asymmetric-effort/convocate/internal/middleware"
)

func Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/events/{applet}/{channel...}", middleware.Chain(
		http.HandlerFunc(handleEvents),
		middleware.Auth,
	))
}

// parseTypeFilter extracts the optional ?filter=type1,type2 query param
// and returns a slice of event types to subscribe to (nil = all).
func parseTypeFilter(r *http.Request) []string {
	raw := r.URL.Query().Get("filter")
	if raw == "" {
		return nil
	}
	var types []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			types = append(types, t)
		}
	}
	return types
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	applet := r.PathValue("applet")
	channel := r.PathValue("channel")
	fullChannel := applet + "/" + channel
	typeFilter := parseTypeFilter(r)

	// WebSocket upgrade
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		// SSE fallback for non-WebSocket clients
		handleSSE(w, r, fullChannel, typeFilter)
		return
	}

	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		return
	}
	defer conn.Close()

	sub := DefaultHub.Subscribe(fullChannel, typeFilter)
	defer DefaultHub.Unsubscribe(fullChannel, sub)

	// Send events to the client
	for {
		select {
		case data := <-sub.ch:
			if err := wsWrite(conn, data); err != nil {
				return
			}
		case <-sub.done:
			return
		case <-time.After(30 * time.Second):
			// Send ping to keep connection alive
			if err := wsWritePing(conn); err != nil {
				return
			}
		}
	}
}

func handleSSE(w http.ResponseWriter, r *http.Request, channel string, typeFilter []string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	sub := DefaultHub.Subscribe(channel, typeFilter)
	defer DefaultHub.Unsubscribe(channel, sub)

	for {
		select {
		case data := <-sub.ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-sub.done:
			return
		case <-r.Context().Done():
			return
		case <-time.After(30 * time.Second):
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

const wsGUID = "258EAFA5-E914-47DA-95CA-5AB5DC085B11"

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return nil, fmt.Errorf("missing key")
	}

	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return nil, fmt.Errorf("no hijacker")
	}

	conn, buf, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"
	buf.WriteString(resp)
	buf.Flush()

	// Start a goroutine to read (and discard) client frames
	go func() {
		wsReadDiscard(conn)
	}()

	return conn, nil
}

func wsWrite(conn net.Conn, data []byte) error {
	frame := make([]byte, 0, 2+8+len(data))
	frame = append(frame, 0x81) // text frame, FIN
	if len(data) < 126 {
		frame = append(frame, byte(len(data)))
	} else if len(data) < 65536 {
		frame = append(frame, 126)
		frame = append(frame, byte(len(data)>>8), byte(len(data)))
	} else {
		frame = append(frame, 127)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(len(data)))
		frame = append(frame, b...)
	}
	frame = append(frame, data...)
	_, err := conn.Write(frame)
	return err
}

func wsWritePing(conn net.Conn) error {
	_, err := conn.Write([]byte{0x89, 0x00}) // ping, no payload
	return err
}

func wsReadDiscard(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		_, err := reader.ReadByte()
		if err != nil {
			if err != io.EOF {
				return
			}
			return
		}
	}
}
