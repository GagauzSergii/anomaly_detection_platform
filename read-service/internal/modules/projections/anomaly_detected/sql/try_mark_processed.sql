-- name: TryMarkProcessed :one
INSERT INTO processed_events (dedup_key, subject, stream_seq)
VALUES ($1, $2, $3)
    ON CONFLICT (dedup_key) DO NOTHING
RETURNING dedup_key;
