-- Node notes (nodes themselves are K8s objects, not DB rows)

CREATE TABLE IF NOT EXISTS node_notes (
    id         SERIAL PRIMARY KEY,
    node_id    TEXT NOT NULL,
    author     TEXT NOT NULL,
    text       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_node_notes_node_id ON node_notes(node_id);
