-- Replay files known to the app and their parse status.
CREATE TABLE replays (
    id             INTEGER PRIMARY KEY,
    path           TEXT    NOT NULL UNIQUE,
    size           INTEGER NOT NULL DEFAULT 0,
    mtime          INTEGER NOT NULL DEFAULT 0,
    status         TEXT    NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'parsing', 'done', 'failed')),
    error          TEXT    NOT NULL DEFAULT '',
    parser_version INTEGER NOT NULL DEFAULT 0,
    match_id       INTEGER,
    added_at       INTEGER NOT NULL,
    parsed_at      INTEGER
);
CREATE INDEX replays_status ON replays (status);

CREATE TABLE matches (
    match_id   INTEGER PRIMARY KEY,
    replay_id  INTEGER NOT NULL UNIQUE REFERENCES replays (id) ON DELETE CASCADE,
    game_build INTEGER NOT NULL,
    game_mode  INTEGER NOT NULL,
    lobby_type INTEGER NOT NULL,
    winner     INTEGER NOT NULL,
    duration   REAL    NOT NULL,
    end_time   INTEGER NOT NULL,
    start_tick INTEGER NOT NULL
);
CREATE INDEX matches_end_time ON matches (end_time);

CREATE TABLE players (
    match_id     INTEGER NOT NULL REFERENCES matches (match_id) ON DELETE CASCADE,
    player_id    INTEGER NOT NULL,
    team         INTEGER NOT NULL,
    team_slot    INTEGER NOT NULL,
    hero_id      INTEGER NOT NULL,
    hero_name    TEXT    NOT NULL,
    name         TEXT    NOT NULL,
    account_id   INTEGER NOT NULL,
    kills        INTEGER NOT NULL,
    deaths       INTEGER NOT NULL,
    assists      INTEGER NOT NULL,
    level        INTEGER NOT NULL,
    last_hits    INTEGER NOT NULL,
    denies       INTEGER NOT NULL,
    net_worth    INTEGER NOT NULL,
    total_gold   INTEGER NOT NULL,
    total_xp     INTEGER NOT NULL,
    hero_damage  INTEGER NOT NULL,
    tower_damage INTEGER NOT NULL,
    healing      INTEGER NOT NULL,
    rank_tier    INTEGER NOT NULL,
    PRIMARY KEY (match_id, player_id)
) WITHOUT ROWID;
CREATE INDEX players_account ON players (account_id);

-- Per-player time series, one row per parser.SeriesStep seconds.
CREATE TABLE series (
    match_id   INTEGER NOT NULL REFERENCES matches (match_id) ON DELETE CASCADE,
    player_id  INTEGER NOT NULL,
    t          REAL    NOT NULL,
    x          REAL    NOT NULL,
    y          REAL    NOT NULL,
    alive      INTEGER NOT NULL,
    level      INTEGER NOT NULL,
    gold       INTEGER NOT NULL,
    net_worth  INTEGER NOT NULL,
    total_gold INTEGER NOT NULL,
    total_xp   INTEGER NOT NULL,
    last_hits  INTEGER NOT NULL,
    denies     INTEGER NOT NULL,
    PRIMARY KEY (match_id, player_id, t)
) WITHOUT ROWID;

-- Point-in-time events (deaths now; purchases, wards, ... later). The
-- type-specific payload lives in data (JSON).
CREATE TABLE events (
    id        INTEGER PRIMARY KEY,
    match_id  INTEGER NOT NULL REFERENCES matches (match_id) ON DELETE CASCADE,
    t         REAL    NOT NULL,
    type      TEXT    NOT NULL,
    player_id INTEGER NOT NULL,
    other_id  INTEGER NOT NULL DEFAULT -1,
    x         REAL,
    y         REAL,
    data      TEXT    NOT NULL DEFAULT '{}'
);
CREATE INDEX events_match_type ON events (match_id, type);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Cached OpenDota constants (heroes, items, abilities), raw JSON.
CREATE TABLE constants (
    name       TEXT PRIMARY KEY,
    data       TEXT    NOT NULL,
    fetched_at INTEGER NOT NULL
);
