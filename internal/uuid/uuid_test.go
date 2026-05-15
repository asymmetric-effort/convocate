package uuid

import (
	"encoding/json"
	"testing"
)

func TestNew(t *testing.T) {
	id := New()
	if id.IsZero() {
		t.Fatal("New() returned zero UUID")
	}
	// Version 4: byte 6 high nibble must be 0x4.
	if (id[6] >> 4) != 4 {
		t.Errorf("version bits: got %x, want 4", id[6]>>4)
	}
	// Variant 10: byte 8 high two bits must be 10.
	if (id[8] >> 6) != 2 {
		t.Errorf("variant bits: got %x, want 2", id[8]>>6)
	}
}

func TestMustNew(t *testing.T) {
	// MustNew should not panic under normal conditions.
	id := MustNew()
	if id.IsZero() {
		t.Fatal("MustNew() returned zero UUID")
	}
}

func TestUniqueness(t *testing.T) {
	seen := make(map[UUID]bool, 1000)
	for range 1000 {
		id := New()
		if seen[id] {
			t.Fatalf("duplicate UUID: %s", id)
		}
		seen[id] = true
	}
}

func TestString(t *testing.T) {
	id := New()
	s := id.String()
	if len(s) != 36 {
		t.Errorf("String() length: got %d, want 36", len(s))
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		t.Errorf("String() format: %q", s)
	}
}

func TestIsZero(t *testing.T) {
	var zero UUID
	if !zero.IsZero() {
		t.Error("zero UUID.IsZero() = false, want true")
	}
	id := MustNew()
	if id.IsZero() {
		t.Error("non-zero UUID.IsZero() = true, want false")
	}
}

func TestParse(t *testing.T) {
	original := MustNew()
	parsed, err := Parse(original.String())
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if parsed != original {
		t.Errorf("Parse(String()) round-trip: got %s, want %s", parsed, original)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"too short", "12345678-1234-1234-1234"},
		{"no dashes", "12345678123412341234123456789012"},
		{"wrong dash positions", "1234567-81234-1234-1234-123456789012"},
		{"invalid hex", "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"},
		{"too long", "12345678-1234-1234-1234-1234567890120"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Parse(testCase.input)
			if err == nil {
				t.Errorf("Parse(%q) expected error, got nil", testCase.input)
			}
		})
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original := MustNew()

	type wrapper struct {
		ID UUID `json:"id"`
	}
	w := wrapper{ID: original}
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded wrapper
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if decoded.ID != original {
		t.Errorf("JSON round-trip: got %s, want %s", decoded.ID, original)
	}
}

func TestJSONUnmarshalInvalid(t *testing.T) {
	type wrapper struct {
		ID UUID `json:"id"`
	}
	var decoded wrapper
	err := json.Unmarshal([]byte(`{"id":"not-a-uuid"}`), &decoded)
	if err == nil {
		t.Error("json.Unmarshal of invalid UUID expected error, got nil")
	}
}

func TestMarshalText(t *testing.T) {
	id := MustNew()
	text, err := id.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText error: %v", err)
	}
	if string(text) != id.String() {
		t.Errorf("MarshalText: got %q, want %q", string(text), id.String())
	}
}
