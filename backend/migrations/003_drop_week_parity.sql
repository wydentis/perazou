-- Week parity (odd/even weeks) is no longer supported: every slot repeats weekly.
ALTER TABLE schedule_slots DROP COLUMN parity;
ALTER TABLE groups DROP COLUMN week1_monday;
