-- Users, groups, roles, and sessions

CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    email       TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    password    TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    idp         TEXT NOT NULL DEFAULT 'local' CHECK (idp IN ('local', 'github')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS groups (
    id      TEXT PRIMARY KEY,
    name    TEXT UNIQUE NOT NULL,
    builtin BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS roles (
    id          TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT '',
    applet      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_groups (
    user_id  TEXT REFERENCES users(id) ON DELETE CASCADE,
    group_id TEXT REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

CREATE TABLE IF NOT EXISTS group_roles (
    group_id TEXT REFERENCES groups(id) ON DELETE CASCADE,
    role_id  TEXT REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, role_id)
);

CREATE TABLE IF NOT EXISTS global_settings (
    id                      INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    require_mfa             BOOLEAN NOT NULL DEFAULT FALSE,
    session_timeout_minutes INTEGER NOT NULL DEFAULT 30,
    password_min_length     INTEGER NOT NULL DEFAULT 12,
    password_rotation_days  INTEGER NOT NULL DEFAULT 90
);

-- Seed builtin groups
INSERT INTO groups (id, name, builtin) VALUES
    ('grp-admins', 'admins', TRUE),
    ('grp-users', 'users', TRUE)
ON CONFLICT DO NOTHING;

-- Seed roles
INSERT INTO roles (id, description, applet) VALUES
    ('admin', 'Full access to all features', 'all'),
    ('node-view', 'View nodes', 'nmgr'),
    ('node-create', 'Provision nodes', 'nmgr'),
    ('node-update', 'Start/stop/edit nodes', 'nmgr'),
    ('node-delete', 'Decommission nodes', 'nmgr'),
    ('agent-view', 'View agents', 'amgr'),
    ('agent-update', 'Create/start/stop/configure agents', 'amgr'),
    ('pb-view', 'View project boards', 'pb'),
    ('pb-update', 'Edit boards, cards, containers, edges', 'pb'),
    ('pb-execute', 'Implement/send cards to agents', 'pb'),
    ('ide-view', 'View projects and files', 'ide'),
    ('ide-update', 'Edit files and create projects', 'ide'),
    ('repo-view', 'View repositories and PRs', 'repo'),
    ('repo-update', 'Create repositories', 'repo'),
    ('repo-merge', 'Merge pull requests', 'repo'),
    ('access-view', 'View users, groups, settings', 'ac'),
    ('access-update', 'Manage users, groups, settings', 'ac'),
    ('support-view', 'View and create tickets', 'sup')
ON CONFLICT DO NOTHING;

-- Map admin role to admins group
INSERT INTO group_roles (group_id, role_id) VALUES ('grp-admins', 'admin')
ON CONFLICT DO NOTHING;

-- Seed default settings
INSERT INTO global_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

-- Seed admin user (password: admin — bcrypt hash)
INSERT INTO users (id, email, name, password) VALUES
    ('usr-admin', 'admin@convocate.local', 'Admin', '$2a$10$rQEY1f8Mh5J5x5x5x5x5xO5x5x5x5x5x5x5x5x5x5x5x5x5x5x')
ON CONFLICT DO NOTHING;

INSERT INTO user_groups (user_id, group_id) VALUES ('usr-admin', 'grp-admins')
ON CONFLICT DO NOTHING;
