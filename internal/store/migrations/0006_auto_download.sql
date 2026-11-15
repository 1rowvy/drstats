-- Auto-downloaded replays removed to respect the storage cap (unix time; 0 =
-- file kept). The match data stays; the replay cannot be re-parsed.
ALTER TABLE replays ADD COLUMN pruned_at INTEGER NOT NULL DEFAULT 0;

-- Outcome of the last auto-download attempt per match.
CREATE TABLE auto_downloads (
    match_id INTEGER PRIMARY KEY,
    status   TEXT    NOT NULL, -- ok / expired / notfound / nosalt / error
    error    TEXT    NOT NULL DEFAULT '',
    tried_at INTEGER NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 1
);
