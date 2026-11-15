-- OpenDota enrichment. Not tied to matches by a foreign key: re-parsing a
-- replay replaces the match rows but the fetched extras stay valid.
CREATE TABLE od_matches (
    match_id   INTEGER PRIMARY KEY,
    status     TEXT    NOT NULL,   -- ok / unparsed / notfound / error
    error      TEXT    NOT NULL DEFAULT '',
    fetched_at INTEGER NOT NULL
);

CREATE TABLE od_players (
    match_id   INTEGER NOT NULL,
    hero_id    INTEGER NOT NULL,
    position   INTEGER NOT NULL DEFAULT 0,
    lane_role  INTEGER NOT NULL DEFAULT 0,
    rank_tier  INTEGER NOT NULL DEFAULT 0,
    party_size INTEGER NOT NULL DEFAULT 0,
    benchmarks TEXT    NOT NULL DEFAULT '{}',
    PRIMARY KEY (match_id, hero_id)
) WITHOUT ROWID;
