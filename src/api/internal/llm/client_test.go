package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestInit_Defaults(t *testing.T) {
	os.Unsetenv("LLM_ENDPOINT")
	os.Unsetenv("LLM_API_KEY")
	os.Unsetenv("LLM_MODEL")

	Init()

	if endpoint != "https://api.anthropic.com/v1/messages" {
		t.Fatalf("expected default endpoint, got %s", endpoint)
	}
	if model != "claude-sonnet-4-20250514" {
		t.Fatalf("expected default model, got %s", model)
	}
}

func TestInit_CustomValues(t *testing.T) {
	os.Setenv("LLM_ENDPOINT", "http://custom:8080/v1/messages")
	os.Setenv("LLM_API_KEY", "test-key")
	os.Setenv("LLM_MODEL", "custom-model")
	defer func() {
		os.Unsetenv("LLM_ENDPOINT")
		os.Unsetenv("LLM_API_KEY")
		os.Unsetenv("LLM_MODEL")
	}()

	Init()

	if endpoint != "http://custom:8080/v1/messages" {
		t.Fatalf("expected custom endpoint, got %s", endpoint)
	}
	if apiKey != "test-key" {
		t.Fatalf("expected test-key, got %s", apiKey)
	}
	if model != "custom-model" {
		t.Fatalf("expected custom-model, got %s", model)
	}
}

func TestEndpoint(t *testing.T) {
	orig := endpoint
	defer func() { endpoint = orig }()

	endpoint = "http://test:1234"
	if Endpoint() != "http://test:1234" {
		t.Fatalf("expected http://test:1234, got %s", Endpoint())
	}
}

func TestSetEndpoint(t *testing.T) {
	orig := endpoint
	defer func() { endpoint = orig }()

	SetEndpoint("http://new:5678")
	if endpoint != "http://new:5678" {
		t.Fatalf("expected http://new:5678, got %s", endpoint)
	}
}

func TestSetAPIKey(t *testing.T) {
	orig := apiKey
	defer func() { apiKey = orig }()

	SetAPIKey("new-key")
	if apiKey != "new-key" {
		t.Fatalf("expected new-key, got %s", apiKey)
	}
}

func TestComplete_NoAPIKey(t *testing.T) {
	apiKey = ""

	_, err := Complete("system", "user")
	if err == nil {
		t.Fatal("expected error when no API key")
	}
	if err.Error() != "LLM_API_KEY not configured" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComplete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("expected x-api-key test-key")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Error("expected anthropic-version 2023-06-01")
		}

		// Verify request body
		var req CompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(req.Messages))
		}
		if req.MaxTokens != 4096 {
			t.Errorf("expected max_tokens 4096, got %d", req.MaxTokens)
		}

		resp := CompletionResponse{
			Content: []ContentBlock{
				{Type: "text", Text: "Hello, world!"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"
	model = "test-model"

	result, err := Complete("system prompt", "user prompt")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if result != "Hello, world!" {
		t.Fatalf("expected 'Hello, world!', got %s", result)
	}
}

func TestComplete_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"

	_, err := Complete("system", "user")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestComplete_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := CompletionResponse{Content: []ContentBlock{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"

	_, err := Complete("system", "user")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if err.Error() != "empty response from LLM" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComplete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"

	_, err := Complete("system", "user")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestComplete_ConnectionError(t *testing.T) {
	endpoint = "http://localhost:1" // unlikely to have anything listening
	apiKey = "test-key"

	_, err := Complete("system", "user")
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestComplete_InvalidEndpointURL(t *testing.T) {
	endpoint = "://\x00bad" // invalid URL triggers http.NewRequest error
	apiKey = "test-key"

	_, err := Complete("system", "user")
	if err == nil {
		t.Fatal("expected error for invalid endpoint URL")
	}
}

func TestComplete_ReadBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100") // lie about content length
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("short")) // write less than declared
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"

	// The io.ReadAll may or may not error depending on the http transport,
	// but the response will be incomplete JSON, triggering parse error at minimum
	_, err := Complete("system", "user")
	if err == nil {
		t.Fatal("expected error for truncated response body")
	}
}
