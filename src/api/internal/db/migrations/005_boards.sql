-- Project boards, containers, cards, and edges

CREATE TABLE IF NOT EXISTS boards (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    repo_id    TEXT REFERENCES repos(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS board_containers (
    id        TEXT PRIMARY KEY,
    board_id  TEXT REFERENCES boards(id) ON DELETE CASCADE,
    title     TEXT NOT NULL,
    agent_id  TEXT,
    minimized BOOLEAN NOT NULL DEFAULT FALSE,
    x         REAL NOT NULL DEFAULT 0,
    y         REAL NOT NULL DEFAULT 0,
    w         REAL NOT NULL DEFAULT 300,
    h         REAL NOT NULL DEFAULT 200
);

CREATE TABLE IF NOT EXISTS board_cards (
    id           TEXT PRIMARY KEY,
    board_id     TEXT REFERENCES boards(id) ON DELETE CASCADE,
    container_id TEXT REFERENCES board_containers(id) ON DELETE SET NULL,
    title        TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'active', 'done', 'fail', 'note')),
    content      TEXT NOT NULL DEFAULT '',
    pos_x        REAL NOT NULL DEFAULT 0,
    pos_y        REAL NOT NULL DEFAULT 0,
    size_w       REAL NOT NULL DEFAULT 200,
    size_h       REAL NOT NULL DEFAULT 120,
    source_refs  TEXT[] DEFAULT '{}',
    note         TEXT
);

CREATE TABLE IF NOT EXISTS board_edges (
    id       TEXT PRIMARY KEY,
    board_id TEXT REFERENCES boards(id) ON DELETE CASCADE,
    type     TEXT NOT NULL CHECK (type IN ('DependsOn', 'RelatesTo')),
    from_id  TEXT REFERENCES board_cards(id) ON DELETE CASCADE,
    to_id    TEXT REFERENCES board_cards(id) ON DELETE CASCADE
);
