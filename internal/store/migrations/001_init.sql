CREATE TABLE users (
  id            INTEGER PRIMARY KEY,
  username      TEXT    NOT NULL UNIQUE,
  password_hash TEXT    NOT NULL,
  display_name  TEXT    NOT NULL DEFAULT '',
  display_link  TEXT    NOT NULL DEFAULT '',
  is_admin      INTEGER NOT NULL DEFAULT 0,
  disabled      INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL
);

CREATE TABLE sessions (
  token_hash TEXT    PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  csrf_token TEXT    NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX sessions_expires ON sessions(expires_at);

CREATE TABLE pastes (
  slug        TEXT    PRIMARY KEY,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title       TEXT    NOT NULL,
  body_html   TEXT    NOT NULL,
  reveal_mode TEXT    NOT NULL CHECK (reveal_mode IN ('immediate', 'tap')),
  expires_at  INTEGER,
  max_views   INTEGER CHECK (max_views IS NULL OR max_views > 0),
  view_count  INTEGER NOT NULL DEFAULT 0,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);
CREATE INDEX pastes_user ON pastes(user_id, created_at DESC);
CREATE INDEX pastes_expires ON pastes(expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
