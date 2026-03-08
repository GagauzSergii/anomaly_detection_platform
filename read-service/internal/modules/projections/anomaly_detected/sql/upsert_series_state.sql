-- name: UpsertSeriesState :exec
INSERT INTO series_state (
    source, metric_name, env, region,
    last_ts, last_value,
    baseline, mad, threshold,
    window_size, threshold_k, detector,
    producer, trace_id,
    event_id, event_version, occurred_at, produced_at
) VALUES (
             $1,$2,$3,$4,
             $5,$6,
             $7,$8,$9,
             $10,$11,$12,
             $13,$14,
             $15,$16,$17,$18
         )
    ON CONFLICT (source, metric_name, env, region)
DO UPDATE SET
    last_ts = EXCLUDED.last_ts,
           last_value = EXCLUDED.last_value,
           baseline = EXCLUDED.baseline,
           mad = EXCLUDED.mad,
           threshold = EXCLUDED.threshold,
           window_size = EXCLUDED.window_size,
           threshold_k = EXCLUDED.threshold_k,
           detector = EXCLUDED.detector,
           producer = EXCLUDED.producer,
           trace_id = EXCLUDED.trace_id,
           event_id = EXCLUDED.event_id,
           event_version = EXCLUDED.event_version,
           occurred_at = EXCLUDED.occurred_at,
           produced_at = EXCLUDED.produced_at,
           updated_at = now();
