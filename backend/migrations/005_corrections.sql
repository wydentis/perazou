-- Correction mode: while the headman keeps it on, students may fix their own marks on past
-- lessons. Every such fix is logged so the headman can see who changed what.
ALTER TABLE groups ADD COLUMN corrections_open BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE attendance_corrections (
    id          BIGSERIAL PRIMARY KEY,
    slot_id     BIGINT NOT NULL REFERENCES schedule_slots (id) ON DELETE CASCADE,
    lesson_date DATE NOT NULL,
    user_id     BIGINT NOT NULL REFERENCES members (user_id) ON DELETE CASCADE,
    old_status  TEXT NOT NULL, -- present | absent | auto (no mark, counted absent)
    new_status  TEXT NOT NULL, -- present | absent
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX attendance_corrections_lesson_idx ON attendance_corrections (slot_id, lesson_date, user_id);
