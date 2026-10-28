CREATE TABLE users (
    id         BIGINT PRIMARY KEY,
    first_name TEXT NOT NULL,
    last_name  TEXT NOT NULL DEFAULT '',
    username   TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE groups (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL,
    invite_code  TEXT NOT NULL UNIQUE,
    timezone     TEXT NOT NULL,
    -- Monday of an odd ("week 1") week; parity of any other date is derived from it.
    week1_monday DATE NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A user belongs to at most one group (user_id is the primary key).
CREATE TABLE members (
    user_id   BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    group_id  BIGINT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    full_name TEXT NOT NULL,
    role      TEXT NOT NULL CHECK (role IN ('headman', 'student')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX members_group_idx ON members (group_id);
CREATE UNIQUE INDEX one_headman_per_group ON members (group_id) WHERE role = 'headman';

CREATE TABLE schedule_slots (
    id         BIGSERIAL PRIMARY KEY,
    group_id   BIGINT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    weekday    SMALLINT NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    start_time TIME NOT NULL,
    end_time   TIME NOT NULL,
    subject    TEXT NOT NULL,
    room       TEXT NOT NULL DEFAULT '',
    -- 0 = every week, 1 = odd weeks, 2 = even weeks
    parity     SMALLINT NOT NULL DEFAULT 0 CHECK (parity IN (0, 1, 2)),
    CHECK (end_time > start_time)
);
CREATE INDEX schedule_slots_group_idx ON schedule_slots (group_id);

CREATE TABLE lesson_cancellations (
    slot_id     BIGINT NOT NULL REFERENCES schedule_slots (id) ON DELETE CASCADE,
    lesson_date DATE NOT NULL,
    PRIMARY KEY (slot_id, lesson_date)
);

CREATE TABLE attendance (
    slot_id     BIGINT NOT NULL REFERENCES schedule_slots (id) ON DELETE CASCADE,
    lesson_date DATE NOT NULL,
    user_id     BIGINT NOT NULL REFERENCES members (user_id) ON DELETE CASCADE,
    status      TEXT NOT NULL CHECK (status IN ('present', 'absent')),
    marked_by   BIGINT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (slot_id, lesson_date, user_id)
);
