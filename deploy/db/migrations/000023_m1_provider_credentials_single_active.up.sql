WITH ranked_active AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY user_id, provider
            ORDER BY updated_at DESC, created_at DESC, id DESC
        ) AS rn
    FROM provider_credentials
    WHERE is_active = TRUE
)
UPDATE provider_credentials pc
SET is_active = FALSE,
    updated_at = now()
FROM ranked_active ranked
WHERE pc.id = ranked.id
  AND ranked.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_credentials_user_provider_single_active
    ON provider_credentials (user_id, provider)
    WHERE is_active = TRUE;
