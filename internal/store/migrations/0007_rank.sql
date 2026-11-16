-- Medal of an account as seen in its OpenDota profile, one row per change
-- (plus the first sighting), to draw the rank over time.
CREATE TABLE od_rank (
    account_id       INTEGER NOT NULL,
    t                INTEGER NOT NULL, -- unix, when it was seen
    rank_tier        INTEGER NOT NULL,
    leaderboard_rank INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, t)
) WITHOUT ROWID;
