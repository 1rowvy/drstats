-- OpenDota match history of an account: basic per-match data for every game,
-- including the ones without a replay. Keyed by account so switching the
-- Steam ID does not mix histories.
CREATE TABLE od_history (
    account_id    INTEGER NOT NULL,
    match_id      INTEGER NOT NULL,
    start_time    INTEGER NOT NULL,
    duration      INTEGER NOT NULL,
    game_mode     INTEGER NOT NULL DEFAULT 0,
    lobby_type    INTEGER NOT NULL DEFAULT 0,
    radiant_win   INTEGER NOT NULL,
    player_slot   INTEGER NOT NULL,
    hero_id       INTEGER NOT NULL,
    kills         INTEGER NOT NULL DEFAULT 0,
    deaths        INTEGER NOT NULL DEFAULT 0,
    assists       INTEGER NOT NULL DEFAULT 0,
    last_hits     INTEGER NOT NULL DEFAULT 0,
    denies        INTEGER NOT NULL DEFAULT 0,
    gpm           INTEGER NOT NULL DEFAULT 0,
    xpm           INTEGER NOT NULL DEFAULT 0,
    hero_damage   INTEGER NOT NULL DEFAULT 0,
    tower_damage  INTEGER NOT NULL DEFAULT 0,
    hero_healing  INTEGER NOT NULL DEFAULT 0,
    lane          INTEGER NOT NULL DEFAULT 0,
    lane_role     INTEGER NOT NULL DEFAULT 0,
    is_roaming    INTEGER NOT NULL DEFAULT 0,
    party_size    INTEGER NOT NULL DEFAULT 0,
    leaver_status INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, match_id)
) WITHOUT ROWID;

-- Last history sync per account.
CREATE TABLE od_history_sync (
    account_id INTEGER PRIMARY KEY,
    synced_at  INTEGER NOT NULL,  -- last attempt (unix)
    ok_at      INTEGER NOT NULL DEFAULT 0, -- last successful sync (unix)
    status     TEXT    NOT NULL,  -- ok / error
    error      TEXT    NOT NULL DEFAULT ''
);
