-- Repositories, pull requests, and IDE projects

CREATE TABLE IF NOT EXISTS repos (
    id              TEXT PRIMARY KEY,
    name            TEXT UNIQUE NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    default_branch  TEXT NOT NULL DEFAULT 'main',
    visibility      TEXT NOT NULL DEFAULT 'private' CONSTRAINT repo_visibility_check CHECK (visibility IN ('private', 'internal', 'public')),
    github_url      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pull_requests (
    id              TEXT PRIMARY KEY,
    repo_id         TEXT REFERENCES repos(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    branch          TEXT NOT NULL,
    target_branch   TEXT NOT NULL DEFAULT 'main',
    status          TEXT NOT NULL DEFAULT 'open' CONSTRAINT pr_status_check CHECK (status IN ('open', 'merged', 'closed')),
    author          TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS projects (
    id                    TEXT PRIMARY KEY,
    name                  TEXT NOT NULL,
    repo_id               TEXT REFERENCES repos(id),
    specification_file_id TEXT NOT NULL DEFAULT 'SPECIFICATION.md',
    board_id              TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
