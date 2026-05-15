package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// UUID represents a 128-bit universally unique identifier (RFC 9562 v4).
type UUID [16]byte

// New generates a random UUID v4 using crypto/rand.
func New() (UUID, error) {
	var id UUID
	_, err := io.ReadFull(rand.Reader, id[:])
	if err != nil {
		return UUID{}, fmt.Errorf("uuid: read crypto/rand: %w", err)
	}
	// Set version 4 (bits 4-7 of byte 6).
	id[6] = (id[6] & 0x0f) | 0x40
	// Set variant 10 (bits 6-7 of byte 8).
	id[8] = (id[8] & 0x3f) | 0x80
	return id, nil
}

// MustNew generates a random UUID v4 and panics on failure.
func MustNew() UUID {
	id, err := New()
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the standard 8-4-4-4-12 hex representation.
func (id UUID) String() string {
	var buf [36]byte
	hex.Encode(buf[0:8], id[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], id[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], id[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], id[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], id[10:16])
	return string(buf[:])
}

// IsZero reports whether the UUID is all zeros.
func (id UUID) IsZero() bool {
	return id == UUID{}
}

// Parse parses a UUID from its 8-4-4-4-12 hex string representation.
func Parse(s string) (UUID, error) {
	if len(s) != 36 {
		return UUID{}, fmt.Errorf("uuid: invalid length %d", len(s))
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return UUID{}, fmt.Errorf("uuid: invalid format")
	}
	hexStr := strings.ReplaceAll(s, "-", "")
	if len(hexStr) != 32 {
		return UUID{}, fmt.Errorf("uuid: invalid hex length %d", len(hexStr))
	}
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return UUID{}, fmt.Errorf("uuid: invalid hex: %w", err)
	}
	var id UUID
	copy(id[:], b)
	return id, nil
}

// MarshalText implements encoding.TextMarshaler for JSON serialization.
func (id UUID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for JSON deserialization.
func (id *UUID) UnmarshalText(data []byte) error {
	parsed, err := Parse(string(data))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}
