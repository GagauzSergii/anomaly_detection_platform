-- name: GetSeriesState :one
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
WHERE source = $1 AND metric_name = $2 AND env = $3 AND region = $4;
