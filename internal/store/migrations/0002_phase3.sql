-- Parser v2: gold sources, support counters, farm/damage series, combat
-- aggregates and keyed events (purchases, casts, hits, wards, runes, ...).
ALTER TABLE players ADD COLUMN gold_creep      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN gold_neutral    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN gold_hero       INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN gold_building   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN gold_passive    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN gold_bounty     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN gold_roshan     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN gold_other      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN gold_lost       INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN buybacks        INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN camps_stacked   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN stuns           REAL    NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN obs_placed      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN sen_placed      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN wards_destroyed INTEGER NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN rune_pickups    INTEGER NOT NULL DEFAULT 0;

ALTER TABLE series ADD COLUMN gold_creep   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE series ADD COLUMN gold_neutral INTEGER NOT NULL DEFAULT 0;
ALTER TABLE series ADD COLUMN dmg_out      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE series ADD COLUMN dmg_in       INTEGER NOT NULL DEFAULT 0;

-- Item / ability / ward kind / rune type of the event, for aggregate queries.
ALTER TABLE events ADD COLUMN key TEXT NOT NULL DEFAULT '';
CREATE INDEX events_type_key ON events (type, key);

-- Damage / healing aggregates per player (see parser.CombatStat).
CREATE TABLE combat (
    match_id  INTEGER NOT NULL REFERENCES matches (match_id) ON DELETE CASCADE,
    player_id INTEGER NOT NULL,
    kind      TEXT    NOT NULL,
    other     TEXT    NOT NULL,
    inflictor TEXT    NOT NULL,
    amount    INTEGER NOT NULL,
    count     INTEGER NOT NULL,
    PRIMARY KEY (match_id, player_id, kind, other, inflictor)
) WITHOUT ROWID;
