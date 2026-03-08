# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-03-08

### Added
- **Core Architecture**: Established a distributed microservices foundation based on **CQRS** and **Event Sourcing** principles.
- **Go Ecosystem**: Initialized high-performance services for Metric Ingestion and Pattern Detection (MAD algorithm).
- **ML Predict Service**: Introduced a Python-based predictive engine featuring **Isolation Forest** (Scikit-learn) and **Autoencoder** (PyTorch) models.
- **Event Bus**: Integrated **NATS JetStream** as the central message broker for persistent, asynchronous inter-service communication.
- **Infrastructure**: Configured a specialized `.gitignore` for a Go/Python monorepo and established SSH aliases for secure multi-account GitHub deployment.
- **Documentation**: Authored a comprehensive README detailing Bounded Contexts, Architectural Principles, and C4-level system flow.

### Changed
- Unified the project structure into a monorepo to streamline collaborative development between Go-based stream processing and Python-based ML inference.

### Fixed
- Resolved SSH authentication conflicts (`Permission denied (publickey)`) by implementing dedicated identity routing in `~/.ssh/config`.

---
*Standard tags used in this project:*
- `Added` for new features.
- `Changed` for changes in existing functionality.
- `Deprecated` for soon-to-be removed features.
- `Removed` for now removed features.
- `Fixed` for any bug fixes.
- `Security` in case of vulnerabilities.