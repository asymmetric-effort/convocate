package llm

import (
	"encoding/json"
	"fmt"

	"github.com/asymmetric-effort/convocate/internal/types"
)

const decomposePrompt = `You are a project decomposition engine. Given a SPECIFICATION.md document, break it down into a Project Board consisting of Containers (work groups mapped to agent-containers) and Cards (individual tasks).

Output a JSON object with this structure:
{
  "containers": [
    {"title": "...", "x": 50, "y": 50, "w": 400, "h": 300}
  ],
  "cards": [
    {"title": "...", "status": "todo", "content": "...", "containerIndex": 0, "x": 70, "y": 80, "w": 200, "h": 120}
  ],
  "edges": [
    {"type": "DependsOn", "fromIndex": 1, "toIndex": 0}
  ]
}

Rules:
- Each container represents a logical work group (e.g., "Backend API", "Frontend", "Database", "Testing")
- Each card represents a concrete, implementable task
- Cards should be granular enough for a single agent to complete
- Use DependsOn edges for sequential dependencies
- Use RelatesTo edges for related but non-blocking tasks
- Position containers and cards so they don't overlap
- Status is always "todo" for new cards

Output ONLY valid JSON, no markdown fences or explanation.`

type decomposedBoard struct {
	Containers []struct {
		Title string  `json:"title"`
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		W     float64 `json:"w"`
		H     float64 `json:"h"`
	} `json:"containers"`
	Cards []struct {
		Title          string  `json:"title"`
		Status         string  `json:"status"`
		Content        string  `json:"content"`
		ContainerIndex int     `json:"containerIndex"`
		X              float64 `json:"x"`
		Y              float64 `json:"y"`
		W              float64 `json:"w"`
		H              float64 `json:"h"`
	} `json:"cards"`
	Edges []struct {
		Type      string `json:"type"`
		FromIndex int    `json:"fromIndex"`
		ToIndex   int    `json:"toIndex"`
	} `json:"edges"`
}

func DecomposeSpec(specContent string) (*types.Board, error) {
	response, err := Complete(decomposePrompt, "Decompose this specification into a Project Board:\n\n"+specContent)
	if err != nil {
		return nil, fmt.Errorf("LLM decomposition: %w", err)
	}

	var decomposed decomposedBoard
	if err := json.Unmarshal([]byte(response), &decomposed); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w (response: %s)", err, response[:min(len(response), 200)])
	}

	board := &types.Board{
		Cards: make([]types.Card, len(decomposed.Cards)),
		Edges: make([]types.Edge, len(decomposed.Edges)),
	}

	// Containers removed — each project has one agent-container
	cardIDs := make([]string, len(decomposed.Cards))
	for i, c := range decomposed.Cards {
		id := fmt.Sprintf("card-%03d", i+1)
		cardIDs[i] = id
		board.Cards[i] = types.Card{
			ID:       id,
			Title:    c.Title,
			Status:   types.CardStatus(c.Status),
			Content:  c.Content,
			Position: &types.Position{X: c.X, Y: c.Y},
			Size:     &types.Size{W: c.W, H: c.H},
			Links:    []types.Edge{},
		}
	}

	for i, e := range decomposed.Edges {
		if e.FromIndex >= 0 && e.FromIndex < len(cardIDs) && e.ToIndex >= 0 && e.ToIndex < len(cardIDs) {
			board.Edges[i] = types.Edge{
				ID:   fmt.Sprintf("edge-%03d", i+1),
				Type: types.EdgeType(e.Type),
				From: cardIDs[e.FromIndex],
				To:   cardIDs[e.ToIndex],
			}
		}
	}

	return board, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
