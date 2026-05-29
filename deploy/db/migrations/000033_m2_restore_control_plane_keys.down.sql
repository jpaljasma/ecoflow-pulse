-- Restore-only migration rollback is intentionally a no-op.
-- The control-plane primary and foreign keys repaired in 000033_m2 are owned
-- by earlier baseline migrations and must remain available.
SELECT 1;
