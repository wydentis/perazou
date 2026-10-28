ALTER TABLE schedule_slots
    ADD COLUMN teacher    TEXT NOT NULL DEFAULT '',
    -- Optional validity window: e.g. a course that only runs 01.09 - 24.10.
    ADD COLUMN valid_from DATE,
    ADD COLUMN valid_to   DATE,
    ADD CONSTRAINT schedule_slots_valid_range CHECK (valid_to IS NULL OR valid_from IS NULL OR valid_to >= valid_from);
