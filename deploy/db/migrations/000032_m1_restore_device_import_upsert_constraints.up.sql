-- Restore unique arbiters required by the provider-device import transaction.
--
-- Older local/cloud data planes may have tables that predate the constraints
-- in 000001/000002. Because those migrations used CREATE TABLE IF NOT EXISTS,
-- drifted tables were not repaired and ON CONFLICT could fail at runtime with
-- SQLSTATE 42P10. Keep the newest active row for any duplicate key before
-- adding the missing arbiters.

DROP TABLE IF EXISTS pg_temp._m32_device_dupes;

CREATE TEMP TABLE _m32_device_dupes AS
WITH ranked AS (
    SELECT
        id,
        first_value(id) OVER (
            PARTITION BY ecoflow_sn
            ORDER BY updated_at DESC, created_at DESC, id DESC
        ) AS keep_device_id,
        row_number() OVER (
            PARTITION BY ecoflow_sn
            ORDER BY updated_at DESC, created_at DESC, id DESC
        ) AS row_rank
    FROM devices
)
SELECT id AS duplicate_device_id, keep_device_id
FROM ranked
WHERE row_rank > 1;

UPDATE devices keep
SET product_name = COALESCE(keep.product_name, duplicate_values.product_name),
    model = COALESCE(keep.model, duplicate_values.model),
    metadata = COALESCE(duplicate_values.metadata, '{}'::jsonb) || keep.metadata,
    updated_at = GREATEST(keep.updated_at, duplicate_values.updated_at)
FROM (
    SELECT DISTINCT ON (dd.keep_device_id)
        dd.keep_device_id,
        d.product_name,
        d.model,
        d.metadata,
        d.updated_at
    FROM _m32_device_dupes dd
    JOIN devices d ON d.id = dd.duplicate_device_id
    ORDER BY
        dd.keep_device_id,
        (d.product_name IS NOT NULL OR d.model IS NOT NULL OR d.metadata <> '{}'::jsonb) DESC,
        d.updated_at DESC,
        d.created_at DESC,
        d.id DESC
) AS duplicate_values
WHERE keep.id = duplicate_values.keep_device_id;

WITH duplicate_admin_links AS (
    SELECT
        ud.user_id,
        dd.keep_device_id,
        MAX(ud.updated_at) AS updated_at
    FROM user_devices ud
    JOIN _m32_device_dupes dd ON dd.duplicate_device_id = ud.device_id
    WHERE ud.role = 'admin'
    GROUP BY ud.user_id, dd.keep_device_id
)
UPDATE user_devices existing
SET role = 'admin',
    updated_at = GREATEST(existing.updated_at, duplicate_admin_links.updated_at)
FROM duplicate_admin_links
WHERE existing.user_id = duplicate_admin_links.user_id
  AND existing.device_id = duplicate_admin_links.keep_device_id
  AND existing.role <> 'admin';

DELETE FROM user_devices ud
USING _m32_device_dupes dd
WHERE ud.device_id = dd.duplicate_device_id
  AND EXISTS (
      SELECT 1
      FROM user_devices existing
      WHERE existing.user_id = ud.user_id
        AND existing.device_id = dd.keep_device_id
  );

UPDATE user_devices ud
SET device_id = dd.keep_device_id
FROM _m32_device_dupes dd
WHERE ud.device_id = dd.duplicate_device_id;

WITH ranked AS (
    SELECT
        pd.id,
        row_number() OVER (
            PARTITION BY COALESCE(dd.keep_device_id, pd.device_id), pd.provider
            ORDER BY
                pd.is_active DESC,
                CASE pd.ingest_desired_state WHEN 'active' THEN 0 WHEN 'draining' THEN 1 ELSE 2 END,
                pd.updated_at DESC,
                pd.created_at DESC,
                pd.id DESC
        ) AS row_rank
    FROM provider_devices pd
    LEFT JOIN _m32_device_dupes dd ON dd.duplicate_device_id = pd.device_id
    WHERE dd.duplicate_device_id IS NOT NULL
       OR EXISTS (
           SELECT 1
           FROM _m32_device_dupes target
           WHERE target.keep_device_id = pd.device_id
       )
)
DELETE FROM provider_devices pd
USING ranked r
WHERE pd.id = r.id
  AND r.row_rank > 1;

UPDATE provider_devices pd
SET device_id = dd.keep_device_id
FROM _m32_device_dupes dd
WHERE pd.device_id = dd.duplicate_device_id;

DELETE FROM devices d
USING _m32_device_dupes dd
WHERE d.id = dd.duplicate_device_id;

WITH ranked AS (
    SELECT
        ctid,
        row_number() OVER (
            PARTITION BY user_id, device_id
            ORDER BY
                CASE role WHEN 'admin' THEN 0 ELSE 1 END,
                updated_at DESC,
                created_at DESC
        ) AS row_rank
    FROM user_devices
)
DELETE FROM user_devices ud
USING ranked r
WHERE ud.ctid = r.ctid
  AND r.row_rank > 1;

WITH ranked AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY provider, provider_device_id
            ORDER BY
                is_active DESC,
                CASE ingest_desired_state WHEN 'active' THEN 0 WHEN 'draining' THEN 1 ELSE 2 END,
                updated_at DESC,
                created_at DESC,
                id DESC
        ) AS row_rank
    FROM provider_devices
)
DELETE FROM provider_devices pd
USING ranked r
WHERE pd.id = r.id
  AND r.row_rank > 1;

WITH ranked AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY device_id, provider
            ORDER BY
                is_active DESC,
                CASE ingest_desired_state WHEN 'active' THEN 0 WHEN 'draining' THEN 1 ELSE 2 END,
                updated_at DESC,
                created_at DESC,
                id DESC
        ) AS row_rank
    FROM provider_devices
)
DELETE FROM provider_devices pd
USING ranked r
WHERE pd.id = r.id
  AND r.row_rank > 1;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_index i
        WHERE i.indrelid = 'devices'::regclass
          AND i.indisunique
          AND i.indisvalid
          AND i.indpred IS NULL
          AND (
              SELECT array_agg(a.attname::text ORDER BY ord.n)
              FROM unnest(i.indkey) WITH ORDINALITY AS ord(attnum, n)
              JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ord.attnum
              WHERE ord.n <= i.indnkeyatts
          ) = ARRAY['ecoflow_sn']
    ) THEN
        ALTER TABLE devices
            ADD CONSTRAINT uq_devices_ecoflow_sn UNIQUE (ecoflow_sn);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_index i
        WHERE i.indrelid = 'user_devices'::regclass
          AND i.indisunique
          AND i.indisvalid
          AND i.indpred IS NULL
          AND (
              SELECT array_agg(a.attname::text ORDER BY ord.n)
              FROM unnest(i.indkey) WITH ORDINALITY AS ord(attnum, n)
              JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ord.attnum
              WHERE ord.n <= i.indnkeyatts
          ) = ARRAY['user_id', 'device_id']
    ) THEN
        ALTER TABLE user_devices
            ADD CONSTRAINT uq_user_devices_user_device UNIQUE (user_id, device_id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'provider_devices'::regclass
          AND conname = 'uq_provider_devices_provider_device_id'
    ) THEN
        ALTER TABLE provider_devices
            ADD CONSTRAINT uq_provider_devices_provider_device_id UNIQUE (provider, provider_device_id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'provider_devices'::regclass
          AND conname = 'uq_provider_devices_device_provider'
    ) THEN
        ALTER TABLE provider_devices
            ADD CONSTRAINT uq_provider_devices_device_provider UNIQUE (device_id, provider);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_provider_devices_device_provider
    ON provider_devices (device_id, provider);

DROP TABLE IF EXISTS pg_temp._m32_device_dupes;
