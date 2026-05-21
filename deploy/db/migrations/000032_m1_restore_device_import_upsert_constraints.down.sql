-- Restore-only migration rollback is intentionally a no-op.
-- The import upsert arbiters repaired in 000032 are owned by earlier control
-- plane and provider integration baselines and must remain available.
SELECT 1;
