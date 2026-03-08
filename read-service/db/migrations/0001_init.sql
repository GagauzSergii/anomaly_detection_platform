-- +goose Up
CREATE TABLE IF NOT EXISTS processed_events (
                                                dedup_key     TEXT PRIMARY KEY,
                                                subject       TEXT NOT NULL,
                                                stream_seq    BIGINT NOT NULL,
                                                processed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
    );

CREATE TABLE IF NOT EXISTS series_state (
                                            source        TEXT NOT NULL,
                                            metric_name   TEXT NOT NULL,
                                            env           TEXT NOT NULL DEFAULT '',
                                            region        TEXT NOT NULL DEFAULT '',

                                            last_ts       BIGINT NOT NULL,
                                            last_value    DOUBLE PRECISION NOT NULL,

                                            baseline      DOUBLE PRECISION NOT NULL,
                                            mad           DOUBLE PRECISION NOT NULL,
                                            threshold     DOUBLE PRECISION NOT NULL,

                                            window_size   INTEGER NOT NULL,
                                            threshold_k   DOUBLE PRECISION NOT NULL,
                                            detector      TEXT NOT NULL,

                                            producer      TEXT NOT NULL DEFAULT '',
                                            trace_id      TEXT NOT NULL DEFAULT '',

                                            event_id      TEXT NOT NULL DEFAULT '',
                                            event_version INTEGER NOT NULL DEFAULT 0,
                                            occurred_at   TEXT NOT NULL DEFAULT '',
                                            produced_at   TEXT NOT NULL DEFAULT '',

                                            updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (source, metric_name, env, region)
    );

CREATE INDEX IF NOT EXISTS idx_series_state_updated_at
    ON series_state (updated_at DESC);

CREATE TABLE IF NOT EXISTS anomaly_events (
                                              id            BIGSERIAL PRIMARY KEY,

                                              source        TEXT NOT NULL,
                                              metric_name   TEXT NOT NULL,
                                              env           TEXT NOT NULL DEFAULT '',
                                              region        TEXT NOT NULL DEFAULT '',
                                              instance_id   TEXT NOT NULL DEFAULT '',

                                              ts            BIGINT NOT NULL,
                                              value         DOUBLE PRECISION NOT NULL,

                                              baseline      DOUBLE PRECISION NOT NULL,
                                              mad           DOUBLE PRECISION NOT NULL,
                                              threshold     DOUBLE PRECISION NOT NULL,

                                              window_size   INTEGER NOT NULL,
                                              threshold_k   DOUBLE PRECISION NOT NULL,
                                              detector      TEXT NOT NULL,

                                              producer      TEXT NOT NULL DEFAULT '',
                                              trace_id      TEXT NOT NULL DEFAULT '',

                                              event_id      TEXT NOT NULL DEFAULT '',
                                              event_type    TEXT NOT NULL DEFAULT '',
                                              event_version INTEGER NOT NULL DEFAULT 0,
                                              occurred_at   TEXT NOT NULL DEFAULT '',
                                              produced_at   TEXT NOT NULL DEFAULT '',

                                              created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
    );

CREATE INDEX IF NOT EXISTS idx_anomaly_events_ts
    ON anomaly_events (ts DESC);

CREATE INDEX IF NOT EXISTS idx_anomaly_events_series_ts
    ON anomaly_events (source, metric_name, env, region, ts DESC);

CREATE INDEX IF NOT EXISTS idx_anomaly_events_instance_ts
    ON anomaly_events (instance_id, ts DESC);

-- +goose Down
DROP TABLE IF EXISTS anomaly_events;
DROP TABLE IF EXISTS series_state;
DROP TABLE IF EXISTS processed_events;
