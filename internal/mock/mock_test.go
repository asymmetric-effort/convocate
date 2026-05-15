package mock

import (
	"testing"
)

func TestDefaultResponses(t *testing.T) {
	responses := DefaultResponses()
	if len(responses) == 0 {
		t.Error("DefaultResponses returned empty slice")
	}
	for i, r := range responses {
		if r.Prompt == "" {
			t.Errorf("response %d has empty prompt", i)
		}
		if r.Response == "" {
			t.Errorf("response %d has empty response", i)
		}
	}
}
