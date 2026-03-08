package config

import "os"

const (
	HTTPAddress      = ":8080"
	PostgresDSN      = "postgres://postgres:postgres@localhost:5432/read?sslmode=disable"
	NATSURL          = "nats://localhost:4222"
	AnomaliesStream  = "ANOMALIES"
	AnomaliesSubject = "anomaly.detected"
	DurableName      = "read-service-anomalies-consumer"
)

// Config holds read-service configuration.
// All values are env-driven (12-factor friendly).
type Config struct {
	HTTPAddr string

	PostgresDSN string
	NATSURL     string

	// Jetstream inputs
	AnomaliesStream  string
	AnomaliesSubject string
	DurableName      string
}

func Load() Config {
	return Config{
		HTTPAddr:         getEnv("HTTP_ADDR", HTTPAddress),
		PostgresDSN:      getEnv("POSTGRES_DSN", PostgresDSN),
		NATSURL:          getEnv("NATS_URL", NATSURL),
		AnomaliesStream:  getEnv("ANOMALIES_STREAM", AnomaliesStream),
		AnomaliesSubject: getEnv("ANOMALIES_SUBJECT", AnomaliesSubject),
		DurableName:      getEnv("DURABLE_NAME", DurableName),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
