package config

import "os"

type Config struct {
	HTTPPort string
	GRPCPort string
	NATSUrl  string
}

func Load() Config {
	port := os.Getenv("METRICS_SERVICE_PORT")
	if port == "" {
		port = "8081"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		// by default for local start without Docker
		natsURL = "nats://localhost:4222"
	}

	return Config{
		HTTPPort: port,
		GRPCPort: grpcPort,
		NATSUrl:  natsURL,
	}
}
