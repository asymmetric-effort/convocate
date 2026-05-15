package protocol

import (
	"testing"
)

func TestClusterAuthModeValid(t *testing.T) {
	tests := []struct {
		mode ClusterAuthMode
		want bool
	}{
		{AuthModeAnthropicKey, true},
		{AuthModeClaudeSession, true},
		{"unknown", false},
		{"", false},
	}
	for _, testCase := range tests {
		t.Run(string(testCase.mode), func(t *testing.T) {
			got := testCase.mode.Valid()
			if got != testCase.want {
				t.Errorf("ClusterAuthMode(%q).Valid() = %v, want %v", testCase.mode, got, testCase.want)
			}
		})
	}
}
