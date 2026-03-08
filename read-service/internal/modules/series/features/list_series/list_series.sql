-- name: ListSeries :many
SELECT
    source,
    metric_name,
    env,
    region,
    last_ts,
    last_value,
    baseline,
    mad,
    threshold,
    window_size,
    threshold_k,
    detector,
    producer,
    trace_id,
    updated_at
FROM series_state
WHERE
    (sqlc.narg('source')::text IS NULL OR source = sqlc.narg('source')) AND
    (sqlc.narg('metric_name')::text IS NULL OR metric_name = sqlc.narg('metric_name')) AND
    (sqlc.narg('env')::text IS NULL OR env = sqlc.narg('env')) AND
    (sqlc.narg('region')::text IS NULL OR region = sqlc.narg('region'))
ORDER BY updated_at DESC
    LIMIT @limit_val;