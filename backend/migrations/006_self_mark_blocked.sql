-- Members the headman marks personally: they cannot mark themselves (app, bot buttons or corrections).
ALTER TABLE members ADD COLUMN self_mark_blocked BOOLEAN NOT NULL DEFAULT false;
