package redis

import (
	"testing"
)

func TestMockConnPing(t *testing.T) {
	mock := NewMockConn()
	val, err := mock.Do("PING")
	if err != nil {
		t.Fatalf("PING error: %v", err)
	}
	if val != "PONG" {
		t.Errorf("got %q, want PONG", val)
	}
}

func TestMockConnSetGet(t *testing.T) {
	mock := NewMockConn()

	_, err := mock.Do("SET", "key1", "value1")
	if err != nil {
		t.Fatalf("SET error: %v", err)
	}

	val, err := mock.Do("GET", "key1")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	if val != "value1" {
		t.Errorf("got %q, want %q", val, "value1")
	}
}

func TestMockConnGetNonexistent(t *testing.T) {
	mock := NewMockConn()
	val, err := mock.Do("GET", "nonexistent")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestMockConnDel(t *testing.T) {
	mock := NewMockConn()
	_, err := mock.Do("SET", "key1", "val1")
	if err != nil {
		t.Fatalf("SET error: %v", err)
	}
	deleted, err := mock.Do("DEL", "key1")
	if err != nil {
		t.Fatalf("DEL error: %v", err)
	}
	if deleted.(int64) != 1 {
		t.Errorf("DEL count: got %d, want 1", deleted)
	}
	val, err := mock.Do("GET", "key1")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil after DEL, got %v", val)
	}
}

func TestMockConnSetOperations(t *testing.T) {
	mock := NewMockConn()

	_, err := mock.Do("SADD", "myset", "a", "b", "c")
	if err != nil {
		t.Fatalf("SADD error: %v", err)
	}

	isMember, err := mock.Do("SISMEMBER", "myset", "b")
	if err != nil {
		t.Fatalf("SISMEMBER error: %v", err)
	}
	if isMember.(int64) != 1 {
		t.Error("SISMEMBER: expected 1 for member 'b'")
	}

	isMember, err = mock.Do("SISMEMBER", "myset", "d")
	if err != nil {
		t.Fatalf("SISMEMBER error: %v", err)
	}
	if isMember.(int64) != 0 {
		t.Error("SISMEMBER: expected 0 for non-member 'd'")
	}

	_, err = mock.Do("SREM", "myset", "b")
	if err != nil {
		t.Fatalf("SREM error: %v", err)
	}

	isMember, err = mock.Do("SISMEMBER", "myset", "b")
	if err != nil {
		t.Fatalf("SISMEMBER error: %v", err)
	}
	if isMember.(int64) != 0 {
		t.Error("SISMEMBER: expected 0 after SREM")
	}
}

func TestMockConnListOperations(t *testing.T) {
	mock := NewMockConn()

	_, err := mock.Do("RPUSH", "mylist", "first")
	if err != nil {
		t.Fatalf("RPUSH error: %v", err)
	}
	_, err = mock.Do("RPUSH", "mylist", "second")
	if err != nil {
		t.Fatalf("RPUSH error: %v", err)
	}

	length, err := mock.Do("LLEN", "mylist")
	if err != nil {
		t.Fatalf("LLEN error: %v", err)
	}
	if length.(int64) != 2 {
		t.Errorf("LLEN: got %d, want 2", length)
	}

	val, err := mock.Do("LPOP", "mylist")
	if err != nil {
		t.Fatalf("LPOP error: %v", err)
	}
	if val != "first" {
		t.Errorf("LPOP: got %q, want %q", val, "first")
	}

	val, err = mock.Do("LPOP", "mylist")
	if err != nil {
		t.Fatalf("LPOP error: %v", err)
	}
	if val != "second" {
		t.Errorf("LPOP: got %q, want %q", val, "second")
	}

	val, err = mock.Do("LPOP", "mylist")
	if err != nil {
		t.Fatalf("LPOP error: %v", err)
	}
	if val != nil {
		t.Errorf("LPOP empty: expected nil, got %v", val)
	}
}

func TestMockConnScan(t *testing.T) {
	mock := NewMockConn()
	_, _ = mock.Do("SET", "router:key1", "val1")
	_, _ = mock.Do("SET", "router:key2", "val2")
	_, _ = mock.Do("SET", "dispatch:key3", "val3")

	result, err := mock.Do("SCAN", "0", "MATCH", "router:*", "COUNT", "100")
	if err != nil {
		t.Fatalf("SCAN error: %v", err)
	}
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(arr) != 2 {
		t.Fatalf("SCAN result length: got %d, want 2", len(arr))
	}
	keys, ok := arr[1].([]interface{})
	if !ok {
		t.Fatalf("expected []interface{} for keys, got %T", arr[1])
	}
	if len(keys) != 2 {
		t.Errorf("matched key count: got %d, want 2", len(keys))
	}
}

func TestMockConnClose(t *testing.T) {
	mock := NewMockConn()
	err := mock.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}
	_, err = mock.Do("PING")
	if err == nil {
		t.Error("expected error after close, got nil")
	}
}

func TestMockConnEmptyCommand(t *testing.T) {
	mock := NewMockConn()
	_, err := mock.Do()
	if err == nil {
		t.Error("expected error for empty command, got nil")
	}
}

func TestMockConnUnknownCommand(t *testing.T) {
	mock := NewMockConn()
	_, err := mock.Do("FOOBAR")
	if err == nil {
		t.Error("expected error for unknown command, got nil")
	}
}

func TestMockConnSetWithEX(t *testing.T) {
	mock := NewMockConn()
	_, err := mock.Do("SET", "key", "val", "EX", "60")
	if err != nil {
		t.Fatalf("SET with EX error: %v", err)
	}
	val, err := mock.Do("GET", "key")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	if val != "val" {
		t.Errorf("got %q, want %q", val, "val")
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		{"*", "anything", true},
		{"foo*", "foobar", true},
		{"foo*", "bar", false},
		{"exact", "exact", true},
		{"exact", "other", false},
	}
	for _, testCase := range tests {
		got := matchGlob(testCase.pattern, testCase.s)
		if got != testCase.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", testCase.pattern, testCase.s, got, testCase.want)
		}
	}
}

func TestMockConnCommandArgErrors(t *testing.T) {
	mock := NewMockConn()

	t.Run("SET too few args", func(t *testing.T) {
		_, err := mock.Do("SET", "key")
		if err == nil {
			t.Error("expected error for SET with too few args")
		}
	})

	t.Run("GET too few args", func(t *testing.T) {
		_, err := mock.Do("GET")
		if err == nil {
			t.Error("expected error for GET with too few args")
		}
	})

	t.Run("DEL too few args", func(t *testing.T) {
		_, err := mock.Do("DEL")
		if err == nil {
			t.Error("expected error for DEL with too few args")
		}
	})

	t.Run("SADD too few args", func(t *testing.T) {
		_, err := mock.Do("SADD", "set")
		if err == nil {
			t.Error("expected error for SADD with too few args")
		}
	})

	t.Run("SREM too few args", func(t *testing.T) {
		_, err := mock.Do("SREM", "set")
		if err == nil {
			t.Error("expected error for SREM with too few args")
		}
	})

	t.Run("SISMEMBER too few args", func(t *testing.T) {
		_, err := mock.Do("SISMEMBER", "set")
		if err == nil {
			t.Error("expected error for SISMEMBER with too few args")
		}
	})

	t.Run("RPUSH too few args", func(t *testing.T) {
		_, err := mock.Do("RPUSH", "list")
		if err == nil {
			t.Error("expected error for RPUSH with too few args")
		}
	})

	t.Run("LPOP too few args", func(t *testing.T) {
		_, err := mock.Do("LPOP")
		if err == nil {
			t.Error("expected error for LPOP with too few args")
		}
	})

	t.Run("LLEN too few args", func(t *testing.T) {
		_, err := mock.Do("LLEN")
		if err == nil {
			t.Error("expected error for LLEN with too few args")
		}
	})
}
