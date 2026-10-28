-- One row per lesson reminder sent, so a reminder goes out at most once per member even
-- across restarts and replicas.
CREATE TABLE lesson_notifications (
    slot_id     BIGINT NOT NULL REFERENCES schedule_slots (id) ON DELETE CASCADE,
    lesson_date DATE NOT NULL,
    user_id     BIGINT NOT NULL REFERENCES members (user_id) ON DELETE CASCADE,
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (slot_id, lesson_date, user_id)
);
