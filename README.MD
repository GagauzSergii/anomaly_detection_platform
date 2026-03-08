# AIOps Anomaly Detection Platform

![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat&logo=go)
![Python Version](https://img.shields.io/badge/Python-3.12-3776AB?style=flat&logo=python)
![Architecture](https://img.shields.io/badge/Architecture-Event--Driven%20%7C%20CQRS-blue)
![Database](https://img.shields.io/badge/PostgreSQL-16-336791?style=flat&logo=postgresql)
![Broker](https://img.shields.io/badge/NATS-JetStream-27AE60?style=flat&logo=nats)

## 1. Executive Summary
The **AIOps Anomaly Detection Platform** is a distributed, real-time system designed for telemetry ingestion, intelligent anomaly detection, and predictive service degradation analysis. 

Engineered for high-performance computing and low-latency processing, the platform's core is built on a microservices architecture utilizing **Domain-Driven Design (DDD)**, **CQRS**, and **Event Sourcing**. The system is capable of ingesting high-throughput metric streams, processing them through rapid statistical analysis pipelines (Go) and asynchronous Machine Learning models (Python), and projecting the results into normalized read models for incident visualization.

## 2. Architectural Principles
The system is designed following strict enterprise engineering standards:
* **Clean Architecture (Ports & Adapters):** Strict isolation of domain logic from infrastructure concerns. No global state, and explicit Dependency Injection.
* **True CQRS:** Physical separation of the Write Model (commands and NATS events) from the Read Model (optimized SQL projections in PostgreSQL).
* **Polyglot & Decoupled:** Ultra-fast stream processing and routing are implemented in Go, while resource-intensive neural network inference is isolated within a dedicated Python environment.
* **Zero Data Loss:** Guaranteed at-least-once delivery achieved via NATS JetStream persistent streams, transactional idempotency checks (`processed_events`), and strict graceful shutdown sequences.

---

## 3. System Components (Bounded Contexts)

### 1) Metrics Feeder (Load & Simulation)
A CLI tool for load generation. It can simulate stable metric baselines, inject spikes (anomalies), and replay historical datasets for pipeline E2E testing and model training.

### 2) Metrics Service (Intelligent Ingress Gateway)
The entry point (gRPC/HTTP) written in Go. 
* **Responsibilities:** Accepts raw data, performs pre-checks (NaN, bounds validation), enriches the payload with context (env, region), and publishes an immutable `metric.recorded` domain event to the message broker. Acts as a highly lightweight and performant gateway.

### 3) Pattern Service (AIOps Engine)
An ultra-fast compute engine (Go) operating entirely in-memory with no dedicated database.
* **Responsibilities:** Subscribes to the `metric.recorded` stream. Evaluates metrics against strict rule chains and rapid statistical algorithms (e.g., MAD — Median Absolute Deviation). Upon breaching a threshold, it instantly publishes an `anomaly.detected` event.

### 4) ML Predict Service (Predictive Engine) 🧠
An asynchronous Python-based service dedicated to predictive analytics and uncovering hidden degradation patterns.
* **Responsibilities:** Operates as an independent Consumer Group. Reads the `metric.recorded` stream from NATS, maintaining a sliding window of historical data in memory.
* **Inference:** Feeds the sliding window into a pre-trained Deep Learning model (e.g., PyTorch/TensorFlow-based LSTM or Transformer). The model evaluates the probability of node failure within the next *N* minutes.
* **Events:** When an upward degradation trend is detected, it generates and publishes a `degradation.predicted` event containing the probability and estimated Time-to-Failure (TTF).
* **Advantages:** Completely isolated from the critical data path. Heavy matrix multiplications do not block the processing of standard statistical anomalies in the Pattern Service.

### 5) Read Service (Projection & Query API)
The read layer (Go) within the CQRS paradigm.
* **Responsibilities:** Listens for `anomaly.detected` and `degradation.predicted` events. Idempotently projects these events into a relational PostgreSQL schema (utilizing `pgxpool` and asynchronous batching). Exposes an optimized HTTP/gRPC API for client dashboards.

---

## 4. System Architecture & Data Flow

### Level 1: C4 Container Diagram
This diagram illustrates the data flow and the isolation of the ML environment from the core statistical engine.

```mermaid
flowchart TD
    Client(["Client / Dashboard"])
    Feeder(["Metrics Feeder"])

    subgraph Write_Path ["Ingestion & Command"]
        MS["Metrics Service<br/>Intelligent Gateway (Go)"]
    end

    subgraph Event_Broker ["Event Broker"]
        NATS[("NATS JetStream<br/>Event Store")]
    end

    subgraph Analysis_Path ["Parallel Analysis Layer"]
        PS["Pattern Service<br/>AIOps Engine / MAD (Go)"]
        ML["ML Predict Service<br/>Deep Learning (Python)"]
    end

    subgraph Read_Path ["Query & Projection"]
        RS["Read Service<br/>Projector & API (Go)"]
        DB[("PostgreSQL<br/>Read Models")]
    end

    Feeder -- "gRPC: RecordMetric" --> MS
    MS -- "Publish: metric.recorded" --> NATS
    
    NATS -- "Subscribe (Stream)" --> PS
    NATS -- "Subscribe (Sliding Window)" --> ML
    
    PS -- "Publish: anomaly.detected" --> NATS
    ML -- "Publish: degradation.predicted" --> NATS
    
    NATS -- "Consume: all incidents" --> RS
    RS -- "SQL: Batch Upsert" --> DB
    
    Client -- "HTTP/gRPC: ListIncidents" --> RS
    RS -- "SQL: Select" --> DB

    classDef service fill:#00ADD8,stroke:#007b99,stroke-width:2px,color:white;
    classDef pythonService fill:#3776AB,stroke:#2a5a82,stroke-width:2px,color:white;
    classDef broker fill:#27AE60,stroke:#1e8449,stroke-width:2px,color:white;
    classDef db fill:#336791,stroke:#234a68,stroke-width:2px,color:white;
    
    class MS,PS,RS service;
    class ML pythonService;
    class NATS broker;
    class DB db;