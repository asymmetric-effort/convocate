package nmgr

import (
	"context"
	"time"

	"github.com/asymmetric-effort/convocate/internal/db"
	"github.com/asymmetric-effort/convocate/internal/types"
)

// pgNoteDB is the production implementation using db.Pool.
type pgNoteDB struct{}

func (pgNoteDB) HasDB() bool { return db.Pool != nil }

// pgRows abstracts the pgx Rows interface for testability.
type pgRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
}

// queryNotes is the function used to execute the list notes query.
// Tests replace it with a mock that returns fake rows.
var queryNotes = func(ctx context.Context, nodeID string) (pgRows, error) {
	return db.Pool.Query(ctx, "SELECT author, created_at, text FROM node_notes WHERE node_id = $1 ORDER BY created_at", nodeID)
}

func (pgNoteDB) ListNotes(ctx context.Context, nodeID string) (notes []types.Note, _ error) {
	rows, err := queryNotes(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var n types.Note
		var t time.Time
		if rows.Scan(&n.Author, &t, &n.Text) == nil {
			n.CreatedAt = t.UTC().Format(time.RFC3339)
			notes = append(notes, n)
		}
	}
	if notes == nil {
		notes = []types.Note{}
	}
	return
}
func (pgNoteDB) AddNote(ctx context.Context, nodeID, author, text string) (createdAt time.Time, _ error) {
	return createdAt, db.Pool.QueryRow(ctx, "INSERT INTO node_notes (node_id, author, text) VALUES ($1, $2, $3) RETURNING created_at", nodeID, author, text).Scan(&createdAt)
}
