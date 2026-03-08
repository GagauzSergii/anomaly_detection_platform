package config

import (
	"os"
	"strconv"
)

type DetectionMode string

const (
	defaultHttpPort      = "8080"
	defaultNatsUrl       = "nats://nats:4222"
	defaultMADWindowSize = 50
	defaultMADWarmup     = 20
	//defaultMadThresholdK = 6.0
	defaultMadThresholdK = 3.0 // for demo

	DetectionModeUp   DetectionMode = "MODE_UP"
	DetectionModeDown DetectionMode = "MODE_DOWN"
	DetectionModeBoth DetectionMode = "MODE_BOTH"

	//defaultMinMAD   = 15.0
	defaultMinMAD   = 1.0 // for demo
	defaultMinDelta = 10.0
)

type Config struct {
	HTTPPort string
	NATSUrl  string

	MADWindowSize int
	MADWarmup     int
	MADThresholdK float64
	MADMode       DetectionMode

	MinMAD   float64
	MinDelta float64
}

func Load() Config {
	return Config{
		HTTPPort: getEnvString("HTTP_PORT", defaultHttpPort),
		NATSUrl:  getEnvString("NATS_URL", defaultNatsUrl),

		MADWindowSize: getEnvInt("MAD_WINDOW_SIZE", defaultMADWindowSize),
		MADWarmup:     getEnvInt("MAD_WARMUP", defaultMADWarmup),
		MADThresholdK: getEnvFloat("MAD_THRESHOLD_K", defaultMadThresholdK),
		MADMode:       getEnvMode("MAD_MODE", DetectionModeBoth),

		MinMAD:   getEnvFloat("MIN_MAD", defaultMinMAD),
		MinDelta: getEnvFloat("MIN_DELTA", defaultMinDelta),
	}
}

func getEnvString(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsedValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsedValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsedValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}
	return parsedValue
}

func getEnvMode(ModeType string, defaultValue DetectionMode) DetectionMode {
	raw := os.Getenv(ModeType)
	if raw == "" {
		return defaultValue
	}

	switch DetectionMode(raw) {
	case DetectionModeUp:
		return DetectionModeUp
	case DetectionModeDown:
		return DetectionModeDown
	case DetectionModeBoth:
		return DetectionModeBoth
	default:
		return defaultValue
	}
}
