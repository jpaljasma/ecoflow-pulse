-- Restore-only migration rollback is intentionally a no-op.
-- The solar forecast constraints and indexes repaired in 000027 are owned by
-- earlier baseline migrations, so rolling back this file must not drop them.
SELECT 1;
