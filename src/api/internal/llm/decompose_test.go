package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecomposeSpec_Success(t *testing.T) {
	decomposed := decomposedBoard{
		Containers: []struct {
			Title string  `json:"title"`
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
			W     float64 `json:"w"`
			H     float64 `json:"h"`
		}{
			{Title: "Backend", X: 50, Y: 50, W: 400, H: 300},
		},
		Cards: []struct {
			Title          string  `json:"title"`
			Status         string  `json:"status"`
			Content        string  `json:"content"`
			ContainerIndex int     `json:"containerIndex"`
			X              float64 `json:"x"`
			Y              float64 `json:"y"`
			W              float64 `json:"w"`
			H              float64 `json:"h"`
		}{
			{Title: "Setup API", Status: "todo", Content: "Create REST API", ContainerIndex: 0, X: 70, Y: 80, W: 200, H: 120},
			{Title: "Add Auth", Status: "todo", Content: "JWT auth", ContainerIndex: 0, X: 70, Y: 220, W: 200, H: 120},
		},
		Edges: []struct {
			Type      string `json:"type"`
			FromIndex int    `json:"fromIndex"`
			ToIndex   int    `json:"toIndex"`
		}{
			{Type: "DependsOn", FromIndex: 1, ToIndex: 0},
		},
	}

	decomposedJSON, _ := json.Marshal(decomposed)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := CompletionResponse{
			Content: []ContentBlock{
				{Type: "text", Text: string(decomposedJSON)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"
	model = "test-model"

	board, err := DecomposeSpec("# My Spec\n\nBuild a REST API")
	if err != nil {
		t.Fatalf("DecomposeSpec: %v", err)
	}

	if len(board.Cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(board.Cards))
	}
	if board.Cards[0].Title != "Setup API" {
		t.Fatalf("expected card title 'Setup API', got %s", board.Cards[0].Title)
	}
	if board.Cards[0].ID != "card-001" {
		t.Fatalf("expected card ID card-001, got %s", board.Cards[0].ID)
	}
	if board.Cards[1].ID != "card-002" {
		t.Fatalf("expected card ID card-002, got %s", board.Cards[1].ID)
	}

	if len(board.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(board.Edges))
	}
	if board.Edges[0].From != "card-002" {
		t.Fatalf("expected edge from card-002, got %s", board.Edges[0].From)
	}
	if board.Edges[0].To != "card-001" {
		t.Fatalf("expected edge to card-001, got %s", board.Edges[0].To)
	}
	if string(board.Edges[0].Type) != "DependsOn" {
		t.Fatalf("expected edge type DependsOn, got %s", board.Edges[0].Type)
	}
}

func TestDecomposeSpec_LLMError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"

	_, err := DecomposeSpec("spec content")
	if err == nil {
		t.Fatal("expected error for LLM failure")
	}
}

func TestDecomposeSpec_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := CompletionResponse{
			Content: []ContentBlock{
				{Type: "text", Text: "not valid json at all"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"

	_, err := DecomposeSpec("spec content")
	if err == nil {
		t.Fatal("expected error for invalid JSON from LLM")
	}
}

func TestDecomposeSpec_InvalidEdgeIndex(t *testing.T) {
	decomposed := decomposedBoard{
		Cards: []struct {
			Title          string  `json:"title"`
			Status         string  `json:"status"`
			Content        string  `json:"content"`
			ContainerIndex int     `json:"containerIndex"`
			X              float64 `json:"x"`
			Y              float64 `json:"y"`
			W              float64 `json:"w"`
			H              float64 `json:"h"`
		}{
			{Title: "Only Card", Status: "todo"},
		},
		Edges: []struct {
			Type      string `json:"type"`
			FromIndex int    `json:"fromIndex"`
			ToIndex   int    `json:"toIndex"`
		}{
			{Type: "DependsOn", FromIndex: 99, ToIndex: 0}, // out of bounds
		},
	}

	decomposedJSON, _ := json.Marshal(decomposed)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := CompletionResponse{
			Content: []ContentBlock{
				{Type: "text", Text: string(decomposedJSON)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"

	board, err := DecomposeSpec("spec")
	if err != nil {
		t.Fatalf("DecomposeSpec: %v", err)
	}
	// Edge with invalid index should result in a zero-value edge
	if board.Edges[0].ID != "" {
		t.Fatal("expected empty edge ID for out-of-bounds index")
	}
}

func TestDecomposeSpec_EmptyCards(t *testing.T) {
	decomposed := decomposedBoard{}
	decomposedJSON, _ := json.Marshal(decomposed)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := CompletionResponse{
			Content: []ContentBlock{
				{Type: "text", Text: string(decomposedJSON)},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	endpoint = server.URL
	apiKey = "test-key"

	board, err := DecomposeSpec("spec")
	if err != nil {
		t.Fatalf("DecomposeSpec: %v", err)
	}
	if len(board.Cards) != 0 {
		t.Fatalf("expected 0 cards, got %d", len(board.Cards))
	}
}

func TestDecomposeSpec_NoAPIKey(t *testing.T) {
	apiKey = ""

	_, err := DecomposeSpec("spec")
	if err == nil {
		t.Fatal("expected error when no API key")
	}
}

func TestMin(t *testing.T) {
	if min(1, 2) != 1 {
		t.Fatal("min(1,2) should be 1")
	}
	if min(3, 2) != 2 {
		t.Fatal("min(3,2) should be 2")
	}
	if min(5, 5) != 5 {
		t.Fatal("min(5,5) should be 5")
	}
}
