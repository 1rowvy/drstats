-- Parser v3: pause intervals (game time -> replay tick for demo_gototick).
ALTER TABLE matches ADD COLUMN pauses TEXT NOT NULL DEFAULT '[]';
-- How long the last successful parse took, for the import page.
ALTER TABLE replays ADD COLUMN parse_ms INTEGER NOT NULL DEFAULT 0;
