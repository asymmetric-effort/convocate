package redis

import (
	"testing"
)

func TestStringHelper(t *testing.T) {
	t.Run("string value", func(t *testing.T) {
		s, err := String("hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "hello" {
			t.Errorf("got %q, want %q", s, "hello")
		}
	})

	t.Run("nil value", func(t *testing.T) {
		s, err := String(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "" {
			t.Errorf("got %q, want empty string", s)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := String(int64(42), nil)
		if err == nil {
			t.Error("expected error for wrong type")
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		_, err := String(nil, &Error{Message: "test"})
		if err == nil {
			t.Error("expected error to propagate")
		}
	})
}

func TestInt64Helper(t *testing.T) {
	t.Run("int64 value", func(t *testing.T) {
		n, err := Int64(int64(42), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 42 {
			t.Errorf("got %d, want 42", n)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		_, err := Int64(nil, nil)
		if err == nil {
			t.Error("expected error for nil value")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := Int64("hello", nil)
		if err == nil {
			t.Error("expected error for wrong type")
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		_, err := Int64(nil, &Error{Message: "test"})
		if err == nil {
			t.Error("expected error to propagate")
		}
	})
}

func TestStringsHelper(t *testing.T) {
	t.Run("string array", func(t *testing.T) {
		arr := []interface{}{"a", "b", "c"}
		result, err := Strings(arr, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 3 {
			t.Fatalf("length: got %d, want 3", len(result))
		}
		if result[0] != "a" || result[1] != "b" || result[2] != "c" {
			t.Errorf("got %v, want [a b c]", result)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		result, err := Strings(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := Strings("not an array", nil)
		if err == nil {
			t.Error("expected error for wrong type")
		}
	})

	t.Run("array with nil element", func(t *testing.T) {
		arr := []interface{}{"a", nil, "c"}
		result, err := Strings(arr, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result[1] != "" {
			t.Errorf("nil element: got %q, want empty string", result[1])
		}
	})

	t.Run("array with wrong element type", func(t *testing.T) {
		arr := []interface{}{"a", int64(42)}
		_, err := Strings(arr, nil)
		if err == nil {
			t.Error("expected error for wrong element type")
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		_, err := Strings(nil, &Error{Message: "test"})
		if err == nil {
			t.Error("expected error to propagate")
		}
	})
}

func TestBoolHelper(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		b, err := Bool(int64(1), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b {
			t.Error("got false, want true")
		}
	})

	t.Run("false", func(t *testing.T) {
		b, err := Bool(int64(0), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b {
			t.Error("got true, want false")
		}
	})

	t.Run("nonzero is true", func(t *testing.T) {
		b, err := Bool(int64(5), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b {
			t.Error("got false, want true for nonzero")
		}
	})
}
