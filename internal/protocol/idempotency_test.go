package protocol

import (
	"testing"
)

func TestIdempotencyKeyString(t *testing.T) {
	key := IdempotencyKey{
		Repository:  "org/repo",
		IssueNumber: 42,
		RunID:       12345,
	}
	want := "org/repo:42:12345"
	got := key.String()
	if got != want {
		t.Errorf("IdempotencyKey.String() = %q, want %q", got, want)
	}
}

func TestIdempotencyKeyValid(t *testing.T) {
	tests := []struct {
		name string
		key  IdempotencyKey
		want bool
	}{
		{
			"valid",
			IdempotencyKey{Repository: "org/repo", IssueNumber: 1, RunID: 100},
			true,
		},
		{
			"valid with issue 0 (ad-hoc)",
			IdempotencyKey{Repository: "org/repo", IssueNumber: 0, RunID: 100},
			true,
		},
		{
			"empty repository",
			IdempotencyKey{Repository: "", IssueNumber: 1, RunID: 100},
			false,
		},
		{
			"zero run_id",
			IdempotencyKey{Repository: "org/repo", IssueNumber: 1, RunID: 0},
			false,
		},
		{
			"negative issue number",
			IdempotencyKey{Repository: "org/repo", IssueNumber: -1, RunID: 100},
			false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := testCase.key.Valid()
			if got != testCase.want {
				t.Errorf("IdempotencyKey.Valid() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestAdHocIdempotencyKey(t *testing.T) {
	key := AdHocIdempotencyKey("org/repo", 99)
	if key.Repository != "org/repo" {
		t.Errorf("Repository = %q, want %q", key.Repository, "org/repo")
	}
	if key.IssueNumber != 0 {
		t.Errorf("IssueNumber = %d, want 0", key.IssueNumber)
	}
	if key.RunID != 99 {
		t.Errorf("RunID = %d, want 99", key.RunID)
	}
	if !key.Valid() {
		t.Error("AdHocIdempotencyKey should be valid")
	}
}

func TestIdempotencyKeyDeterministic(t *testing.T) {
	key := IdempotencyKey{Repository: "org/repo", IssueNumber: 5, RunID: 200}
	first := key.String()
	second := key.String()
	if first != second {
		t.Errorf("IdempotencyKey.String() not deterministic: %q != %q", first, second)
	}
}
