-- name: ListAnomalies :many
SELECT
    id,
    source,
    metric_name,
    env,
    region,
    instance_id,
    ts,
    value,
    baseline,
    mad,
    threshold,
    window_size,
    threshold_k,
    detector,
    producer,
    trace_id,
    event_id,
    created_at
FROM anomaly_events
WHERE
    (sqlc.narg('from_ts')::bigint IS NULL OR ts >= sqlc.narg('from_ts')) AND
    (sqlc.narg('to_ts')::bigint IS NULL OR ts <= sqlc.narg('to_ts')) AND
    (sqlc.narg('source')::text IS NULL OR source = sqlc.narg('source')) AND
    (sqlc.narg('metric_name')::text IS NULL OR metric_name = sqlc.narg('metric_name')) AND
    (sqlc.narg('env')::text IS NULL OR env = sqlc.narg('env')) AND
    (sqlc.narg('region')::text IS NULL OR region = sqlc.narg('region')) AND
    (sqlc.narg('instance_id')::text IS NULL OR instance_id = sqlc.narg('instance_id'))
ORDER BY ts DESC
    LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
