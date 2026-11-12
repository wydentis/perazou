-- Remember the reminder message so its buttons can be removed once the marking window closes.
ALTER TABLE lesson_notifications ADD COLUMN message_id BIGINT;
ALTER TABLE lesson_notifications ADD COLUMN closed BOOLEAN NOT NULL DEFAULT false;
