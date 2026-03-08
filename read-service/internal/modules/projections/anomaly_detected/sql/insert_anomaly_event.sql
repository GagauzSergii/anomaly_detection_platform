-- name: InsertAnomalyEvent :exec
INSERT INTO anomaly_events (
    source, metric_name, env, region, instance_id,
    ts, value, baseline, mad, threshold,
    window_size, threshold_k, detector, producer,
    trace_id, event_id, event_type, event_version,
    occurred_at, produced_at
) VALUES (
             $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
             $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
         );