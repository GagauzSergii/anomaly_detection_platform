# Service Golden Path — Anomaly Detection Platform

### Go 1.24 · Enterprise Production Engineering Standard · v2.0

> **Philosophy:** Zero data loss through layered defence. Simplicity through explicitness. Observability as a first-class citizen.  
> **Source of Truth:** `anomaly_detection_platform` codebase, February 2026.  
> **Audience:** Every engineer building or reviewing a Go microservice at Anomaly Detection Platform.

> [!CAUTION]
> This document is normative. Deviations require an ADR (Architecture Decision Record) approved by the platform team lead.

---

## Table of Contents

1. [Project Structure & Layering Rules](#1-project-structure--layering-rules)
2. [Dependency Injection & Wiring](#2-dependency-injection--wiring)
3. [Concurrency & Graceful Shutdown](#3-concurrency--graceful-shutdown)
4. [HTTP Ingress — net/http + chi](#4-http-ingress--nethttp--chi)
5. [NATS JetStream Messaging](#5-nats-jetstream-messaging)
6. [Database — PostgreSQL (pgx/v5)](#6-database--postgresql-pgxv5)
7. [Redis](#7-redis)
8. [Idempotency](#8-idempotency)
9. [Distributed Tracing & TraceID](#9-distributed-tracing--traceid)
10. [Observability — Logging & Metrics](#10-observability--logging--metrics)
11. [Error Handling](#11-error-handling)
12. [Build, Docker & Kubernetes](#12-build-docker--kubernetes)
13. [Code Quality & Linting](#13-code-quality--linting)
14. [Testing Strategy](#14-testing-strategy)
15. [Git, Versioning & CI/CD](#15-git-versioning--cicd)
16. [SLOs, Alerts & Operational Readiness](#16-slos-alerts--operational-readiness)
17. [Decision Log](#17-decision-log)
18. [Future Architecture — Transactional Outbox](#18-future-architecture--transactional-outbox)
19. [Quick-Start Checklist](#19-quick-start-checklist)

---

## 1. Project Structure & Layering Rules

Every service in the Anomaly Detection Platform follows the same directory layout. This is not an arbitrary convention — it is an enforceable contract. A new engineer joining any team can open any repository and immediately know where to find the HTTP handlers, where the database adapter lives, and where to add a new intent handler. Predictability reduces cognitive load and speeds up code review.

We use **Ports and Adapters** (Hexagonal Architecture) to isolate business logic from infrastructure concerns. The core idea is that the innermost packages (`internal/domain`, `pkg/model`) define **interfaces** (ports) and have no knowledge of Postgres, Redis, or NATS. The outer layers (`internal/infra/`) **implement** those interfaces (adapters). This separation makes it trivial to swap an infrastructure adapter — for example, moving from Redis to Memcached for deduplication — without touching a single line of business logic.

```
service-name/
├── cmd/
│   └── service-name/
│       └── main.go              # Composition Root ONLY — no business logic
├── internal/
│   ├── domain/                  # Ports (interfaces) + domain types
│   │   ├── ports.go             # MessageStore, Deduplicator, Publisher, Sequencer
│   │   └── errors.go            # Domain-level sentinel errors
│   ├── app/                     # Application services (orchestrate domain + ports)
│   ├── classify/                # Domain: ML + regex classification engine
│   ├── config/                  # Environment-based configuration loading
│   ├── handler/                 # HTTP handlers — depend on domain ports only
│   ├── infra/                   # Infrastructure adapters (implement domain ports)
│   │   ├── postgres.go
│   │   ├── redis.go
│   │   └── nats.go
│   ├── middleware/              # HTTP middleware (RequestID, logging, recovery)
│   ├── observability/           # Logger and Prometheus metrics factories
│   ├── preprocess/              # Channel-specific ingress adapters
│   ├── sources/                 # NATS consumer setup per source channel
│   └── store/                   # Batch writer, DLQ manager, record types
├── pkg/
│   ├── model/                   # Canonical domain models — ZERO internal imports
│   └── normalizer/              # Pipeline, Handler interface, handler registry
├── migrations/                  # Plain SQL files, sequentially numbered
├── .golangci.yml
├── Makefile
├── Dockerfile
├── VERSION                      # Plain semver, e.g. 1.4.2
└── .env.sample                  # Placeholder values only — never real credentials
```

### Dependency Rules (Strict Ports & Adapters)

The table below defines which packages are allowed to import which others. Violating these rules — for example, importing `internal/infra` from `internal/domain` — will be caught by `depguard` in `golangci-lint` and will block the CI build.

> [!IMPORTANT]
> The Domain/Logic layer **MUST NEVER** import `internal/infra`. The domain defines the interfaces (ports); the infrastructure layer implements them. The Composition Root (`cmd/main.go`) is the only place where concrete adapters are instantiated and injected into domain/application services.

| Layer | Package | May Import | MUST NOT Import |
|---|---|---|---|
| **Composition Root** | `cmd/` | Everything | — |
| **Domain (Ports)** | `internal/domain/` | `pkg/model`, stdlib only | `internal/infra`, `internal/store` |
| **Application** | `internal/app/` | `internal/domain/`, `pkg/model` | `internal/infra` directly |
| **Domain Logic** | `internal/classify/`, `pkg/normalizer/` | `internal/domain/`, `pkg/model` | `internal/infra` |
| **Infrastructure (Adapters)** | `internal/infra/`, `internal/store/` | Third-party drivers, `internal/domain/` | — |
| **Shared Contracts** | `pkg/model/` | **Standard library only** | Everything else |
| **HTTP** | `internal/handler/` | `internal/domain/`, `pkg/model` | `internal/infra` |

#### Domain Ports Example

```go
// internal/domain/ports.go

package domain

import (
    "context"
    "github.com/metrics-service/pkg/model"
)

// MessageStore persists raw inbound messages. Implemented by internal/store.
type MessageStore interface {
    EnqueueRaw(ctx context.Context, rec *model.InboundRawRecord) error
    Flush(ctx context.Context) (int, error)
}

// Deduplicator checks whether a message has been seen before.
type Deduplicator interface {
    Check(ctx context.Context, key string) (isNew bool, err error)
    Reset(ctx context.Context, key string) error
}

// Publisher sends normalized envelopes to the message bus.
type Publisher interface {
    PublishJSON(ctx context.Context, subject string, payload any) error
    Close() error
}

// Sequencer assigns monotonically increasing sequence numbers.
type Sequencer interface {
    Next(ctx context.Context, tenantID string) (string, error)
}
```

This structure guarantees that `internal/app/` and `internal/classify/` work exclusively against interfaces. You can test the entire business logic pipe with in-memory stubs — no Postgres, no Redis, no network.

---

## 2. Dependency Injection & Wiring

We deliberately avoid dependency injection frameworks (Wire, Dig, fx). At the scale we operate, the explicit wiring in `main.go` is easy to read, easy to debug, and carries zero magic. When something is wrong at startup — a missing dependency, a misconfigured pool — the error appears immediately at the call site, not somewhere inside a framework reflection chain.

The `cmd/<service>/main.go` file is the **Composition Root**: the single place where the entire object graph is assembled. It has exactly one job: build things and connect them. No business logic belongs here. The construction order matters and is intentional — infrastructure before application services, application services before the pipeline, pipeline before the HTTP server.

> [!CAUTION]
> **`init()` functions are banned project-wide.** Self-registration of handlers, side-effect imports (`_ "pkg/handlers"`), or any dependency wiring via `init()` makes the startup sequence implicit and untestable. All handler registration and routing MUST be done explicitly in the composition root. This is enforced by the `gochecknoinits` linter.

```go
// cmd/metrics-service/main.go

func main() {
    // 1. Context bound to OS signals — the SINGLE root of all cancellation
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    // 2. Configuration from environment
    cfg, err := config.Load()
    if err != nil {
        log.Fatal("load config", err)
    }

    // 3. Logger (first dependency — everything else uses it)
    logger := observability.NewLogger(observability.Config{
        Level:   cfg.LogLevel,
        Format:  cfg.LogFormat,
        Service: cfg.ServiceName,
        Env:     cfg.Env,
    })
    defer observability.Sync(logger)

    // 4. Infrastructure adapters (order matters for shutdown)
    pgPool, err := infra.NewPostgres(ctx, cfg, logger)
    if err != nil { logger.Fatalw("postgres init", "error", err) }

    redisClient, err := infra.NewRedis(ctx, cfg, logger)
    if err != nil { logger.Fatalw("redis init", "error", err) }

    natsConn, err := infra.NewNats(ctx, cfg, logger)
    if err != nil { logger.Fatalw("nats init", "error", err) }

    // 5. Application services — wired against domain.Port interfaces
    metrics    := observability.NewMetricsRegistry(cfg.ServiceName)
    pgWriter   := store.NewWriter(pgPool, logger, cfg.BatchSize, cfg.FlushEvery) // implements domain.MessageStore
    dedupe     := infra.NewDedupe(redisClient, logger, cfg.DedupeTTL)            // implements domain.Deduplicator
    sequencer  := infra.NewSequencer(redisClient, logger, true)                  // implements domain.Sequencer
    publisher  := infra.NewPublisher(natsConn, logger)                           // implements domain.Publisher
    classifier := classify.NewHybridClassifier(redisClient, cfg.MLServiceURL, logger)

    // 6. Explicit handler registration — NO init() magic
    normalizer.RegisterHandler("rfq.create",   handlers.NewRFQCreateHandler(logger))
    normalizer.RegisterHandler("quote.accept",  handlers.NewQuoteAcceptHandler(logger))
    normalizer.RegisterHandler("order.submit",  handlers.NewOrderSubmitHandler(logger))

    // 7. Core pipeline
    pipeline := normalizer.NewPipeline(logger, metrics, classifier, sequencer, dedupe, publisher, pgWriter)

    // 8. Start background services — pass the SAME ctx for unified shutdown
    pgWriter.Start(ctx)

    // 9. HTTP server (net/http + chi)
    srv := handler.NewServer(handler.Config{
        Addr:     cfg.HTTPAddr,
        Pipeline: pipeline,
        Logger:   logger,
        Version:  cfg.Version,
    })

    // 10. Block until signal
    if err := srv.ListenAndServe(ctx); err != nil {
        logger.Errorw("server error", "error", err)
    }

    // 11. Ordered shutdown (CRITICAL — writer drains before pool closes)
    logger.Info("shutdown initiated")
    publisher.Close()
    pgWriter.Stop()      // drain buffer → final flush → then safe to close pool
    redisClient.Close()
    pgPool.Close()
    natsConn.Drain()
    natsConn.Close()

    logger.Info("shutdown complete")
}
```

**Rules:**
- Every constructor accepts its dependencies as explicit parameters — interfaces from `internal/domain/`, never concrete adapter types.
- No global state. No `init()` side effects. Period.
- `pgWriter.Stop()` **must** be called before `pgPool.Close()` to prevent data loss on restart.
- The single `ctx` from `signal.NotifyContext` is the root of all goroutine lifecycles.

---

## 3. Concurrency & Graceful Shutdown

Go makes concurrency easy to write but easy to get wrong. Goroutine leaks — goroutines that run forever after the process receives `SIGTERM` — are one of the most common sources of subtle data corruption and resource exhaustion in production Go services.

The central idea is that **every goroutine in the service is bound to the single root `context.Context`**. When the top-level context is cancelled by OS signal, the cancellation propagates automatically through every sub-context. Goroutines that respect `ctx.Done()` exit cleanly. Infrastructure connections that accept a context drain their in-flight operations. No data is lost if the shutdown sequence is followed correctly.

### Signal Handling

```go
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()
// All goroutines receive this ctx. SIGTERM cancels it, propagating shutdown.
```

### Unified Context Tree

> [!WARNING]
> **Never call `context.WithCancel(context.Background())` inside a background service.** This creates a context disconnected from the parent shutdown tree. The goroutine will not receive `SIGTERM` and will hang during rolling deployments, potentially losing in-memory data.

```go
// ❌ WRONG — disconnected from shutdown
func (w *Writer) Start() {
    ctx, cancel := context.WithCancel(context.Background())  // orphaned context!
    w.cancel = cancel
    go w.run(ctx)
}

// ✅ CORRECT — derived from parent
func (w *Writer) Start(ctx context.Context) {
    w.ctx, w.cancel = context.WithCancel(ctx)  // child of signal context
    w.wg.Add(1)
    go w.run(w.ctx)
}
```

### HTTP Server Lifecycle

```go
func (s *Server) ListenAndServe(ctx context.Context) error {
    srv := &http.Server{Addr: s.addr, Handler: s.router}

    errCh := make(chan error, 1)
    go func() { errCh <- srv.ListenAndServe() }()

    select {
    case err := <-errCh:
        return fmt.Errorf("http server: %w", err)
    case <-ctx.Done():
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        return srv.Shutdown(shutdownCtx)  // drains in-flight requests
    }
}
```

### Background Writer Stop Sequence

```go
func (w *Writer) Stop() {
    w.cancel()   // signal run() loop to exit
    w.wg.Wait()  // wait for final flush to complete — blocks until all batches are persisted
}
```

### Goroutine Rules

| Rule | Rationale |
|---|---|
| Every goroutine derives context from the root `ctx` | Unified shutdown tree — no orphaned goroutines |
| `wg.Add(1)` before `go` / `wg.Done()` in deferred call | Guarantees `Stop()` waits for completion |
| Never use `context.Background()` in inflight work | Ignores SIGTERM — data loss on shutdown |
| Fire-and-forget banned for data-path operations | Only acceptable for best-effort telemetry |
| `natsConn.Drain()` before `Close()` | Flushes pending publishes before disconnecting |

### Shutdown Order (Mandatory)

```
1. Cancel root context                — stops accepting new work
2. publisher.Close()                  — flush pending NATS publishes
3. pgWriter.Stop()                    — drain buffer → final batch flush to DB
4. redisClient.Close()                — close Redis connections
5. pgPool.Close()                     — close DB pool (writer already drained)
6. natsConn.Drain() + natsConn.Close() — NATS graceful disconnect
```

Reversing steps 3 and 5 — closing the pool before the writer drains — causes the final batch of in-memory records to be discarded, resulting in **silent data loss on every deployment**.

---

## 4. HTTP Ingress

### Framework Policy: Two Options

#### PRIMARY: `net/http` + `chi` (Recommended)

`net/http` + `chi` is the **primary standard** for all new services. It provides:

- **100% native `context.Context` compatibility** — OpenTelemetry spans, gRPC metadata, and cancellation signals flow natively through every handler and middleware without any adapter layer.
- **OTel integration** — `otelhttp.NewHandler()` wraps the router directly. Span propagation works out of the box with the standard `traceparent` header.
- **Ecosystem access** — `httprate`, `chi/middleware`, `alice`, `cors` — all target `net/http.Handler`.

#### ACCEPTABLE ALTERNATIVE: Fiber (v2/v3)

Fiber may be used in existing services or where its performance characteristics are needed. However, engineers **must** be aware of and mitigate the following:

> [!WARNING]
> Fiber is built on `fasthttp`, which uses `*fasthttp.RequestCtx` instead of `context.Context`. When using Fiber, you **must** explicitly bridge `*fiber.Ctx` to `context.Context` in every handler that calls downstream services, publishes to NATS, or interacts with OpenTelemetry:
>
> ```go
> // ✅ Required pattern in Fiber handlers
> func handler(c *fiber.Ctx) error {
>     ctx := c.UserContext()  // bridge to standard context
>     span := trace.SpanFromContext(ctx)  // OTel works
>     result, err := service.Process(ctx, input)  // propagates correctly
>     // ...
> }
> ```
>
> Failure to bridge contexts breaks the distributed tracing chain and gRPC span propagation silently — traces will appear fragmented in Jaeger/Tempo.

| Criterion | `net/http` + `chi` | Fiber v2/v3 |
|---|---|---|
| `context.Context` | Native | Requires `c.UserContext()` bridge |
| OpenTelemetry | `otelhttp.NewHandler()` — zero config | `otelfiber` adapter — requires care |
| gRPC interop | Native context propagation | Manual bridging required |
| Middleware ecosystem | Largest (standard `http.Handler`) | Smaller, Fiber-specific |
| Raw throughput | Excellent | Slightly higher (fasthttp) |
| **Recommendation** | **PRIMARY** | **Acceptable with caveats** |

### Server Configuration (chi)

```go
// internal/handler/server.go

import (
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/httprate"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func NewServer(cfg Config) *Server {
    r := chi.NewRouter()

    // ──────────────────────────────────────────────────────
    // Middleware order is CRITICAL — see §4 Middleware Order
    // ──────────────────────────────────────────────────────
    r.Use(middleware.RequestID)                      // 1. Inject X-Request-ID / generate UUID
    r.Use(middleware.RealIP)                         // 2. Extract real client IP
    r.Use(otelhttp.NewMiddleware(cfg.ServiceName))   // 3. OpenTelemetry span creation + traceparent extraction
    r.Use(NewStructuredLogger(cfg.Logger))           // 4. Structured access log WITH trace_id from context
    r.Use(middleware.Recoverer)                      // 5. Panic recovery
    r.Use(middleware.Compress(5))                    // 6. Gzip
    r.Use(corsMiddleware())                          // 7. CORS

    // 8. Rate limiting — per-IP, prevents DDoS and accidental spam
    r.Use(httprate.LimitByIP(200, 1*time.Minute))   // 200 req/min per IP

    // Admin routes (no rate limit — internal only)
    r.Group(func(r chi.Router) {
        r.Get("/admin/healthz", handleHealthz)
        r.Get("/admin/readyz",  handleReadyz(cfg))
        r.Get("/admin/version", handleVersion(cfg.Version))
        r.Get("/admin/stats",   handleStats)
    })

    // Ingress routes — rate-limited, payload-limited
    r.Group(func(r chi.Router) {
        r.Post("/ingress/rest",      NewRESTHandler(cfg.Pipeline, cfg.Logger))
        r.Post("/ingress/slack",     NewSlackHandler(cfg.Pipeline, cfg.Logger))
        r.Post("/ingress/telegram",  NewTelegramHandler(cfg.Pipeline, cfg.Logger))
        r.Post("/ingress/whatsapp",  NewWhatsAppHandler(cfg.Pipeline, cfg.Logger))
        r.Get("/ingress/ws",         NewWSHandler(cfg.Pipeline, cfg.Logger))
    })

    return &Server{router: r, addr: cfg.Addr}
}
```

### Rate Limiting

> [!IMPORTANT]
> Every HTTP ingress **must** have rate limiting to prevent DDoS, accidental spam from upstream services, and subsequent OOM crashes. Limits must be configurable per environment.

```go
// Per-IP rate limiting (default)
r.Use(httprate.LimitByIP(200, 1*time.Minute))

// Per-tenant rate limiting (if tenant is extracted from header/token)
r.Use(httprate.Limit(
    500,                    // requests
    1*time.Minute,          // window
    httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
        return r.Header.Get("X-Tenant-ID"), nil
    }),
    httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusTooManyRequests, map[string]string{
            "error": "rate limit exceeded",
        })
    }),
))
```

### Middleware Order

> [!IMPORTANT]
> `RequestID` middleware **MUST** be registered **BEFORE** the logger middleware. This guarantees that the `X-Request-ID` is available in `context.Context` when the logger middleware runs, so every log line carries `trace_id`. Reversing this order produces log entries without trace context — making incident investigation impossible.

```go
// internal/middleware/request_id.go

func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" {
            id = uuid.NewString()
        }
        ctx := context.WithValue(r.Context(), RequestIDKey, id)
        w.Header().Set("X-Request-ID", id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// internal/middleware/logger.go — reads RequestID from context

func NewStructuredLogger(logger *zap.SugaredLogger) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            reqID, _ := r.Context().Value(RequestIDKey).(string)

            ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
            next.ServeHTTP(ww, r)

            logger.Infow("http request",
                "trace_id",  reqID,         // ← available because RequestID ran first
                "method",    r.Method,
                "path",      r.URL.Path,
                "status",    ww.Status(),
                "latency_ms", time.Since(start).Milliseconds(),
            )
        })
    }
}
```

### Payload Size Limits

> [!CAUTION]
> Bounding the writer's channel size (e.g., 1000 items) is **not sufficient OOM protection** if JSON payloads vary wildly in size. A single 50MB payload can exhaust memory. Every ingress handler **must** enforce `http.MaxBytesReader` to cap the request body.

```go
const maxRequestBodySize = 1 << 20  // 1 MiB — configurable via env

func limitBody(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
        next.ServeHTTP(w, r)
    })
}
```

### HTTP 202 Response — Minimal Acknowledgment

> [!WARNING]
> **Never echo the full request body in the response.** Doing so leaks PII into client logs and monitoring. Return only a minimal acknowledgment.

```go
func NewRESTHandler(pipeline *normalizer.Pipeline, log *zap.SugaredLogger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        reqID, _ := r.Context().Value(middleware.RequestIDKey).(string)

        // Enforce payload size limit
        r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

        var input InputEnvelope
        if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
            writeJSON(w, http.StatusBadRequest, map[string]string{
                "error":      err.Error(),
                "request_id": reqID,
            })
            return
        }

        msg, err := preprocess.Transform(r.Context(), input)
        if err != nil {
            writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
                "error":      "preprocess failed",
                "request_id": reqID,
            })
            return
        }

        msg.Meta["request_id"] = reqID
        pipeline.Submit(r.Context(), msg)

        // ✅ Minimal acknowledgment — never echo the full body
        writeJSON(w, http.StatusAccepted, map[string]string{
            "status":     "accepted",
            "request_id": reqID,
        })
    }
}
```

### Route Table

| Method | Path | Description |
|---|---|---|
| `POST` | `/ingress/rest` | Generic REST message intake |
| `POST` | `/ingress/slack` | Slack events |
| `POST` | `/ingress/telegram` | Telegram messages |
| `POST` | `/ingress/whatsapp` | WhatsApp messages |
| `GET` | `/ingress/ws` | WebSocket upgrade |
| `GET` | `/admin/healthz` | Liveness probe |
| `GET` | `/admin/readyz` | Readiness probe (checks DB + Redis + NATS) |
| `GET` | `/admin/version` | Build info |
| `GET` | `/admin/stats` | Runtime stats (goroutines, memory) |

---

## 5. NATS JetStream Messaging

NATS JetStream is the primary inter-service communication backbone. Unlike plain NATS (fire-and-forget pub/sub), JetStream provides **at-least-once delivery** through persistent streams and explicit consumer acknowledgements. This is essential for financial message processing where loss of a trade intent or quote command is unacceptable.

Every stream is backed by file storage (not memory) to survive broker restarts. Consumers are always **durable** — they have a deterministic name so that after a service restart the consumer reconnects to the same position in the stream rather than starting over from the beginning.

### Subject Naming Convention

```
<type>.<domain>.<action>.v<version>

ingress.raw.<channel>.v1   — raw inbound before normalization
cmd.<domain>.<action>.v1   — normalized commands
evt.<domain>.<action>.v1   — events and error notifications

Examples:
  ingress.raw.whatsapp.v1
  cmd.rfq.create.v1
  cmd.quote.accept.v1
  evt.normalized.v1
  evt.error.classification.v1
```

### PublishJSON with 3-Retry Linear Backoff

Network blips between the service and the NATS broker are inevitable — especially during rolling deployments. We use **linear backoff** (not exponential) because NATS reconnections are typically fast (under a second), and exponential backoff would introduce unnecessary latency on the critical path.

The retry loop waits for a JetStream `PubAck` — confirmation from the broker that the message has been written to the stream.

```go
// internal/infra/nats.go — implements domain.Publisher

const maxRetries = 3

func (p *NATSPublisher) PublishJSON(ctx context.Context, subject string, payload any) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("marshal payload: %w", err)
    }

    var lastErr error
    for i := 0; i < maxRetries; i++ {
        if i > 0 {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(time.Duration(i) * 500 * time.Millisecond):
            }
        }
        _, err = p.js.Publish(ctx, subject, data)
        if err == nil {
            return nil
        }
        lastErr = err
        p.logger.Warnw("nats publish retry", "subject", subject, "attempt", i+1, "error", err)
    }
    return fmt.Errorf("nats publish after %d retries: %w", maxRetries, lastErr)
}
```

> **Use `PublishJSON`** for all commands and events.  
> **Never** pass `context.Background()` — always propagate the caller's `ctx`.

### JetStream Consumer — Callback Acknowledgement Pattern

> [!CAUTION]
> **Messages must ONLY be `Ack()`'d AFTER the corresponding batch has been successfully committed to PostgreSQL.** The consumer **must not** call `m.Ack()` directly. Instead, it passes the `m.Ack` and `m.Nak` functions as **closures** inside a `BatchItem`. The batch writer calls `OnAck()` after a successful `tx.Commit()` (or a terminal failure), and `OnNak()` if the commit fails with a transient error.

```go
// internal/store/batch_item.go

// BatchItem carries a record AND its NATS acknowledgement callbacks.
// The writer calls OnAck/OnNak — the consumer never calls m.Ack() directly.
type BatchItem struct {
    Record  Record
    OnAck   func()  // called after successful tx.Commit
    OnNak   func()  // called on DB failure — triggers NATS re-delivery
}
```

The consumer creates a `BatchItem` for each message, embedding the NATS acknowledgement functions:

```go
// internal/sources/raw_consumer.go

consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
    Durable:       "metrics-service",
    AckPolicy:     jetstream.AckExplicitPolicy,
    FilterSubject: "ingress.raw.whatsapp.v1",
    AckWait:       30 * time.Second,  // must exceed batch flush interval
    MaxDeliver:    5,                 // dead-letter after 5 failed delivery attempts
})

for i := 0; i < concurrency; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            select {
            case <-ctx.Done():
                return
            default:
            }
            msgs, err := consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
            if err != nil {
                if ctx.Err() != nil { return }
                if strings.Contains(err.Error(), "timeout") { continue }
                time.Sleep(time.Second)
                continue
            }
            for m := range msgs.Messages() {
                // ── Explicit variable shadowing for closure safety ──
                // Go 1.24 fixes loop variable capture, but explicit
                // shadowing remains a best practice for code review
                // clarity and developer peace of mind.
                msg := m

                start := time.Now()
                result, err := pipeline.Process(ctx, msg.Data())
                if err != nil {
                    msg.Nak()
                    metrics.IncCounter("pipeline_errors_total", "stage", "process")
                    continue
                }

                // Pass Ack/Nak as closures — the writer decides when to call them
                item := store.BatchItem{
                    Record: *result,
                    OnAck:  func() {
                        msg.Ack()  // uses shadowed 'msg', safe in closure
                        metrics.ObserveLatency("normalize_latency_seconds", time.Since(start))
                    },
                    OnNak: func() { msg.Nak() },  // uses shadowed 'msg'
                }

                // Enqueue blocks until accepted (backpressure) or ctx cancelled
                if err := pgWriter.Enqueue(ctx, item); err != nil {
                    msg.Nak()  // context cancelled — ensure re-delivery
                }
                // NOTE: msg.Ack() is NOT called here — only the writer calls OnAck
            }
        }
    }()
}
```

---

## 6. Database — PostgreSQL (pgx/v5)

We use **pgx/v5** with `pgxpool` as the Postgres driver. pgx supports the Postgres wire protocol natively (faster than `database/sql`), provides first-class support for Postgres-specific types (JSONB, UUID, arrays), and `pgxpool` handles connection lifecycle, health checks, and reconnection automatically.

All Postgres interactions in the service are **asynchronous by default**. Business logic never blocks on a database write directly. Instead, records are handed to the `store.Writer`, which accumulates them in memory and flushes in batches.

### Connection Pool

The pool is configured conservatively: a minimum of 1 connection to avoid cold-start latency and a maximum of 10 to prevent overwhelming the database. The `application_name` parameter is attached so every connection is identifiable in `pg_stat_activity`.

```go
// internal/infra/postgres.go

func NewPostgres(ctx context.Context, cfg *config.Config, log *zap.SugaredLogger) (*pgxpool.Pool, error) {
    poolCfg, err := pgxpool.ParseConfig(cfg.PostgresURL)
    if err != nil {
        return nil, fmt.Errorf("parse postgres dsn: %w", err)
    }

    poolCfg.MinConns                          = 1
    poolCfg.MaxConns                          = int32(cfg.PGMaxConns) // default 10
    poolCfg.HealthCheckPeriod                 = 30 * time.Second
    poolCfg.MaxConnLifetime                   = 30 * time.Minute
    poolCfg.MaxConnIdleTime                   = 10 * time.Minute
    poolCfg.ConnConfig.ConnectTimeout         = 5 * time.Second
    poolCfg.ConnConfig.RuntimeParams["application_name"] = cfg.ServiceName

    pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
    if err != nil {
        return nil, fmt.Errorf("create pgx pool: %w", err)
    }

    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("postgres ping: %w", err)
    }
    log.Infow("postgres connected", "dsn", sanitizeDSN(cfg.PostgresURL))
    return pool, nil
}

func sanitizeDSN(dsn string) string {
    if idx := strings.Index(dsn, "@"); idx != -1 {
        return "***REDACTED***" + dsn[idx:]
    }
    return dsn
}
```

### Transactions — WithTx Helper

The `WithTx` helper encapsulates the boilerplate of beginning a transaction, deferring a rollback (which is a no-op if `Commit` succeeds), and returning unified error handling.

```go
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)

    if err := fn(tx); err != nil {
        return err
    }
    return tx.Commit(ctx)
}
```

**Always use `$1, $2` placeholders — never string-interpolated SQL.**

### Schema Overview

```
ingress schema
├── inbound_raw       — immutable audit log of every received message
│     dedupe_key TEXT UNIQUE  ← idempotency constraint
├── normalized        — canonical CommandEnvelope after pipeline
├── error_log         — structured per-stage error records
└── dlq               — dead-letter queue for UNPROCESSABLE messages only
```

All migrations use `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` — they are idempotent and safe to re-run.

```bash
# Apply migrations (recommended: use golang-migrate for tracking)
migrate -path migrations/ -database "$DATABASE_URL" up
```

### Async Batch Writer — Zero Data Loss with Callback Ack

> [!CAUTION]
> **Messages must NEVER be silently dropped.** The `select/default` drop pattern is **BANNED**. The writer applies backpressure (blocks the caller). NATS `Ack()` is called **only** after `tx.Commit()` succeeds.

The writer operates on `BatchItem`s (not raw `Record`s). Each item carries `OnAck` and `OnNak` callbacks from the NATS consumer. After a successful PostgreSQL commit, the writer iterates the batch and calls `item.OnAck()` for every item. If the commit fails with a transient error, it calls `item.OnNak()` so NATS re-delivers every message in the failed batch. If it fails with a terminal error, it calls `item.OnAck()` and writes to the DLQ.

```go
// internal/store/pg_writer.go — implements domain.MessageStore

type Writer struct {
    queue        chan BatchItem
    pool         *pgxpool.Pool
    dlq          *DLQManager
    logger       *zap.SugaredLogger
    ctx          context.Context
    cancel       context.CancelFunc
    wg           sync.WaitGroup
    inflightBytes prometheus.Gauge  // tracks actual memory footprint
}

func NewWriter(pool *pgxpool.Pool, log *zap.SugaredLogger, batchSize int, metrics *MetricsRegistry) *Writer {
    return &Writer{
        queue: make(chan BatchItem, batchSize*10), // bounded — backpressure, not drops
        pool:  pool,
        dlq:   NewDLQManager(pool, log),
        logger: log,
        inflightBytes: metrics.NewGauge("inflight_bytes",
            "Approximate memory footprint of records in the writer buffer"),
    }
}

func (w *Writer) Start(ctx context.Context) {
    w.ctx, w.cancel = context.WithCancel(ctx)
    w.wg.Add(1)
    go w.run()
}

func (w *Writer) run() {
    defer w.wg.Done()
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    batch := make([]BatchItem, 0, 100)

    flush := func() {
        if len(batch) == 0 { return }
        if err := w.writeBatch(w.ctx, batch); err != nil {
            w.logger.Errorw("batch flush failed", "count", len(batch), "error", err)

            if isTransient(err) {
                // Transient error: PostgreSQL down or network timeout.
                // Nak all — NATS will re-deliver. DO NOT write to DLQ.
                for _, item := range batch {
                    if item.OnNak != nil { item.OnNak() }
                }
            } else {
                // Terminal error: Batch constraint violation or unprocessable record.
                // Ack all to drop from main NATS queue, and write the batch to DLQ.
                for _, item := range batch {
                    if item.OnAck != nil { item.OnAck() }
                }
                w.dlq.WriteBatch(w.ctx, batch, err.Error())
            }
        } else {
            // ✅ Commit succeeded — Ack every message in the batch
            for _, item := range batch {
                if item.OnAck != nil { item.OnAck() }
            }
        }
        // Update memory gauge
        w.inflightBytes.Set(0)
        batch = batch[:0]
    }

    for {
        select {
        case item := <-w.queue:
            batch = append(batch, item)
            w.inflightBytes.Add(float64(len(item.Record.Payload)))
            if len(batch) >= 100 {
                flush()
            }
        case <-ticker.C:
            flush()
        case <-w.ctx.Done():
            // Drain remaining items before final flush
            for {
                select {
                case item := <-w.queue:
                    batch = append(batch, item)
                default:
                    goto done
                }
            }
        done:
            flush()
            return
        }
    }
}

func (w *Writer) Stop() {
    w.cancel()
    w.wg.Wait()
}

// Enqueue blocks until accepted or context cancelled. NEVER drops.
func (w *Writer) Enqueue(ctx context.Context, item BatchItem) error {
    select {
    case w.queue <- item:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

**Key properties:**
- `OnAck()` is called **only** after `tx.Commit()` succeeds — zero data loss.
- `OnNak()` is called on transient DB failure — NATS re-delivers every message in the failed batch.
- Terminal batch failures trigger `OnAck()` + DLQ write (never a redelivery loop).
- `Enqueue` **blocks** when the buffer is full (backpressure).
- `inflight_bytes` Prometheus gauge tracks the actual memory footprint of the buffer, not just item count.
- On shutdown, remaining items are drained and flushed before the pool closes.

### DLQ Semantics — Transient vs. Terminal Errors

> [!CAUTION]
> **Conflating transient and terminal errors in DLQ handling is a critical operational mistake.** Sending transient failures to the DLQ pollutes it with valid, retryable messages — making real poison pills impossible to find. Conversely, Nak'ing a poison pill causes infinite redelivery loops that block the consumer.

All consumer errors fall into exactly two categories:

| Error Category | Examples | Consumer Action |
|---|---|---|
| **Transient** (infrastructure will recover) | PostgreSQL down, connection refused, Redis timeout, network blip | `Nak()` or `NakWithDelay()` **ONLY** — do **NOT** write to DLQ |
| **Terminal** (message is unprocessable) | Invalid JSON, schema validation failure, unknown intent (Poison Pill), `MaxDeliver` exceeded | `Ack()` to drop from main queue **AND** write to DLQ |

**Transient errors → Nak only:**
```go
// PostgreSQL is temporarily down — the message itself is valid.
// Nak triggers NATS re-delivery. The message will succeed when PG recovers.
result, err := pipeline.Process(ctx, msg.Data())
if isTransient(err) {
    msg.NakWithDelay(5 * time.Second)
    metrics.IncCounter("pipeline_errors_total",
        "stage", telemetry.StagePersistence,
        "error_type", telemetry.ErrTypeDBTimeout,
    )
    continue  // ← NO DLQ write
}
```

**Terminal errors (Poison Pills) → Ack + DLQ:**
```go
// Unprocessable JSON — retrying will never fix this message.
// Ack() removes it from the main queue; DLQ preserves it for investigation.
result, err := pipeline.Process(ctx, msg.Data())
if isTerminal(err) {
    msg.Ack()  // drop from main queue
    dlq.Write(ctx, msg.Data(), err.Error())
    metrics.IncCounter("dlq_records_total", "reason", "terminal_error")
    continue
}
```

> [!IMPORTANT]
> A DB `UNIQUE` constraint violation on a deduplication key is **NOT** a DLQ event — it is successful duplicate detection (see §8).

```go
dlq := store.NewDLQManager(pool, log)

records, _ := dlq.FetchOldest(ctx, 50)
dlq.Delete(ctx, record.ID)
dlq.PurgeOlderThan(ctx, 7*24*time.Hour)
dlq.RetryFailedBatch(ctx, 20, retryFunc)
```

### Hard Payload Size Limits in NATS Consumers

> [!CAUTION]
> The HTTP `MaxBytesReader` (§4) protects the ingress layer, but direct NATS consumers have no such gate. A single oversized message can OOM the writer buffer. **Defence-in-depth requires a strict size check at the very beginning of the NATS consumer pipeline.**

If a message exceeds the configured limit, it is treated as a **Poison Pill**: `Ack()` to remove it from the stream, write to DLQ for investigation, and increment a dedicated `dropped_oversized` metric.

```go
const maxPayloadSize = 1 << 20  // 1 MiB — must match HTTP ingress limit

// At the TOP of the consumer loop, before any processing:
for msg := range msgs.Messages() {
    if len(msg.Data()) > maxPayloadSize {
        msg.Ack()  // remove from stream — retrying won't shrink the payload
        dlq.Write(ctx, msg.Data()[:1024], "payload_exceeds_max_size")  // truncate for DLQ
        metrics.IncCounter("dropped_oversized_total",
            "stream", streamName,
        )
        logger.Warnw("oversized NATS message dropped",
            "size_bytes", len(msg.Data()),
            "max_bytes",  maxPayloadSize,
            "stream",     streamName,
        )
        continue
    }

    // ... normal processing pipeline ...
}
```

**Rules:**
- `maxPayloadSize` must be a named constant — never a magic number inline.
- The limit should match or be lower than the HTTP ingress `maxRequestBodySize` to ensure consistency.
- Only the first 1 KiB of the oversized payload is written to DLQ (truncated) to prevent the DLQ itself from OOMing.
- The `dropped_oversized_total` metric is a bounded counter (by stream name only) and must trigger a `HighOversizedDrop` alert (see §16).

---

## 7. Redis

Redis is used as an **operational data store**, not a cache. The distinction is important: data stored in Redis (dedup keys, sequence counters, classifier rules) is written with the expectation that it will be read shortly after, but its loss does not cause data corruption — the database `UNIQUE` constraint is the permanent safety net.

All three uses have fundamentally different key-lifetime characteristics. Dedup keys are short-lived (minutes) because the window of duplicate delivery is narrow. Sequence counters live for one calendar day. Classifier rules have no TTL because they are admin-managed configuration.

### Key Namespace

| Prefix | Full Pattern | Purpose | TTL |
|---|---|---|---|
| `dedupe:` | `dedupe:<sha256hex>` | Idempotency gate | `DEDUPE_TTL` (default 5m) |
| `seq:tenant:` | `seq:tenant:<id>:<YYYYMMDD>` | Daily per-tenant counter | UTC midnight + 5m |
| `seq:global:` | `seq:global:<YYYYMMDD>` | Daily global counter | UTC midnight + 5m |
| `classifier:intents:` | `classifier:intents:<channel>` | Hot-reloadable rules (JSON) | No expiry |

### SETNX Deduplication Gate — PII-Safe Keys

The deduplication check is the **first operation** in the normalization pipeline — before classification, sequencing, or any Postgres write. We discard duplicates at the earliest possible moment to avoid wasting CPU and database connections on messages that will ultimately be rejected.

> [!CAUTION]
> **Deduplication keys MUST NOT contain raw text, payload content, or any PII.** Using `rawText` as a hash input creates two problems: (1) PII appears in Redis `MONITOR` traces and memory dumps, and (2) hash instability — whitespace normalization, encoding differences, or payload reformatting causes the same logical message to produce different hashes, defeating deduplication entirely.
>
> Use only **stable, canonical identifiers** from the message envelope.

```go
// internal/infra/dedupe.go — implements domain.Deduplicator

func (d *RedisDedupe) Check(ctx context.Context, key string) (bool, error) {
    redisKey := "dedupe:" + key
    isNew, err := d.redis.SetNX(ctx, redisKey, 1, d.ttl).Result()
    if err != nil {
        return false, fmt.Errorf("redis setnx: %w", err)
    }
    return isNew, nil
}
```

The dedup key is computed in the application layer using only stable envelope fields:

```go
// internal/app/pipeline.go

func computeDedupeKey(msg model.RawMessage) string {
    // ✅ Stable canonical IDs only — NO rawText, NO payload content
    raw := msg.TenantID + "|" + msg.ClientID + "|" + msg.Channel + "|" +
           msg.SessionID + "|" + msg.ConversationID + "|" + msg.EnvelopeID
    hash := sha256.Sum256([]byte(raw))
    return fmt.Sprintf("%x", hash)
}
```

### Daily Sequencer with Random Offset

Every normalized message is assigned a sequence number of the form `YYYYMMDD:N`. The counter resets at UTC midnight. To prevent external parties from inferring message volume, the counter is initialized with a **random offset** between 1,000 and 10,999 on first use each day.

```go
// internal/infra/sequencer.go — implements domain.Sequencer

func (s *RedisSequencer) Next(ctx context.Context, tenantID string) (string, error) {
    date := time.Now().UTC().Format("20060102")
    key  := fmt.Sprintf("seq:tenant:%s:%s", tenantID, date)

    val, err := s.redis.Incr(ctx, key).Result()
    if err != nil {
        return "", fmt.Errorf("redis incr: %w", err)
    }

    if val == 1 && s.useOffset {
        offset := int64(cryptoRandInt(10000) + 1000)  // crypto/rand — see §13 Security
        val, _ = s.redis.IncrBy(ctx, key, offset).Result()
        s.redis.ExpireAt(ctx, key, nextMidnightUTC().Add(5*time.Minute))
    }

    return fmt.Sprintf("%s:%d", date, val), nil
}
```

### Hot-Reload Intent Rules

Classifier rules are stored in Redis so they can be updated without redeploying the service. During incidents, a misconfigured classification rule can be corrected immediately via `redis-cli` without a full build-deploy cycle.

```go
redisClient.StoreIntentRules(ctx, "whatsapp", rules)
rules, err := redisClient.LoadIntentRules(ctx, "whatsapp")
```

```bash
# Inspect intent rules
redis-cli GET "classifier:intents:whatsapp"

# NEVER use KEYS in production — always SCAN
redis-cli SCAN 0 MATCH "dedupe:*" COUNT 100
```

### Redis Production Rules

| Rule | Reason |
|---|---|
| All keys must have a TTL (except classifier rules) | Prevent unbounded memory growth |
| Use `SetNX` for all idempotent writes | Avoids race conditions |
| Use `SCAN` not `KEYS *` | `KEYS` blocks the single-threaded Redis server |
| Use `REDIS_DB` per environment (0=prod, 1=staging, 2=dev) | Namespace isolation |
| Never store PII in Redis keys or values | Keys appear in `MONITOR` and RDB dumps |

---

## 8. Idempotency

Idempotency is one of the most critical requirements in financial message processing. If a trade intent is processed twice — two RFQs created, two quotes accepted — the consequences are real: duplicate orders, incorrect positions, potential financial loss. The system must guarantee exactly-once processing even in the face of network retries, service restarts, and broker redeliveries.

We implement a **defence-in-depth** strategy: three independent layers, each covering a different failure mode.

```
Inbound Message
      │
      ▼
┌────────────────────┐
│  Redis SETNX gate  │── duplicate ──► Ack + bump dedup_redis_total
└────────┬───────────┘
         │ new
         ▼
┌────────────────────┐
│ NATS AckExplicit   │── Nak (fail) ──► re-deliver (Redis catches it)
└────────┬───────────┘
         │ processing succeeded
         ▼
┌─────────────────────────┐
│ DB UNIQUE dedupe_key    │── violation ──► Ack + bump dedup_db_total
└─────────────────────────┘   (NOT a DLQ event!)
         │ inserted
         ▼
    Persisted ✓ → Ack
```

| Layer | Mechanism | TTL / Scope |
|---|---|---|
| L1 | `Redis SetNX` on SHA256(canonical IDs) | `DEDUPE_TTL` (default 5 min) |
| L2 | `NATS AckExplicitPolicy` + `Nak` on failure | Until consumer's `AckWait` expires |
| L3 | `dedupe_key TEXT UNIQUE` on `ingress.inbound_raw` | Permanent |

### Correct DLQ Semantics

> [!IMPORTANT]
> A DB `UNIQUE` constraint violation on `dedupe_key` is **successful duplicate detection** — the system is working correctly. The correct response is:
> 1. **`Ack()` the NATS message** (the message was handled — it's a known duplicate)
> 2. **Increment `dedup_db_total` metric** (observability)
> 3. **Do NOT send to DLQ** (the message is not malformed)
>
> DLQ is reserved exclusively for terminal errors:
> - Messages that fail schema validation / JSON parsing
> - Records that fail batch persistence due to terminal data errors (e.g., constraint violations other than dedupe)
> - Messages that exceed `MaxDeliver` on the NATS consumer (5 attempts)

> [!WARNING]
> **DEPRECATED** — kept for historical reference to illustrate `ON CONFLICT`; does not match current Ack-after-commit callback model; do not copy.

```go
func (w *Writer) writeBatch(ctx context.Context, batch []Record) error {
    tx, err := w.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin batch tx: %w", err)
    }
    defer tx.Rollback(ctx)

    for _, rec := range batch {
        _, err := tx.Exec(ctx,
            `INSERT INTO ingress.inbound_raw (tenant_id, client_id, channel, dedupe_key, payload)
             VALUES ($1, $2, $3, $4, $5)
             ON CONFLICT (dedupe_key) DO NOTHING`,
            rec.TenantID, rec.ClientID, rec.Channel, rec.DedupeKey, rec.Payload,
        )
        if err != nil {
            return fmt.Errorf("insert raw: %w", err)
        }
    }

    return tx.Commit(ctx)
}
```

The `ON CONFLICT (dedupe_key) DO NOTHING` clause handles duplicates silently at the database level. The write succeeds (no error), the duplicate row is simply ignored, and the metric counter is bumped based on `rows affected == 0`.

---

## 9. Distributed Tracing & TraceID

The service uses **lightweight correlation tracing** today, with a schema that is forward-compatible with OpenTelemetry. Every request entering the system is assigned a `TraceID` (UUIDv4), which is propagated through all downstream services via the `CommandEnvelope` and `ErrorEnvelope`.

### TraceID Lifecycle

```
HTTP Request
  └── X-Request-ID header (UUIDv4, injected by RequestID middleware)
        │
        ├── stored in context.Context via middleware.RequestIDKey
        │
        ▼
  Preprocessor extracts from context → RawMessage.Meta["request_id"]
        │
        ▼
  MessageHandler injects into CommandEnvelope.TraceID
        │
        ▼
  NATS envelope carries trace_id through all downstream services
        │
        └── ErrorEnvelope.TraceID (same value — errors share the trace)
```

### TraceID Population

```go
func (h *MessageHandler) ProcessRaw(ctx context.Context, msg model.RawMessage) error {
    traceID, _ := msg.Meta["request_id"].(string)
    if traceID == "" {
        traceID = uuid.NewString()
    }

    envelope := model.CommandEnvelope{
        EnvelopeID: uuid.NewString(),
        TraceID:    traceID,
        TenantID:   msg.TenantID,
        ClientID:   msg.ClientID,
        Intent:     decision.Intent,
        Schema:     "cmd." + decision.Intent + ".v1",
        Payload:    result.Payload,
        Sequence:   seqNum,
        Source:     msg.Channel,
    }

    h.logger.Infow("envelope built",
        "trace_id", traceID,
        "intent",   decision.Intent,
        "seq",      seqNum,
    )

    return h.publisher.PublishJSON(ctx, "evt.normalized.v1", envelope)
}
```

### Cross-Service Log Queries

```bash
# All events for a single request across all services
{ trace_id = "3f2a1b4c-..." }

# All errors for a tenant in the last hour
{ tenant = "tenant-01" AND level = "error" }
```

### OpenTelemetry — Native Integration

The `TraceID` field maps directly to the W3C `traceparent` header. Our architecture requires **native OTel integration** at every ingress point.

> [!IMPORTANT]
> **`traceparent` extraction is mandatory at every ingress** — HTTP and NATS. Strict context propagation must be maintained through all layers: `Ingress → Application Logic → NATS Publish → PostgreSQL Write`. If any layer passes `context.Background()` instead of the request context, the trace chain breaks and spans appear orphaned in Jaeger/Tempo.

**HTTP Ingress:**
```go
// Already handled by otelhttp.NewMiddleware() in §4 middleware stack.
// The middleware extracts traceparent from incoming requests and injects
// a span into context. All downstream code receives it via r.Context().
```

**NATS Ingress (consumer side):**
```go
// Extract traceparent from NATS message headers
carrier := propagation.HeaderCarrier(msg.Headers())
ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
ctx, span := tracer.Start(ctx, "nats.consume", trace.WithSpanKind(trace.SpanKindConsumer))
defer span.End()

// Pass ctx through the entire pipeline — never context.Background()
result, err := pipeline.Process(ctx, msg.Data())
```

**NATS Publish (producer side):**
```go
// Inject traceparent into NATS message headers
headers := nats.Header{}
carrier := propagation.HeaderCarrier(headers)
otel.GetTextMapPropagator().Inject(ctx, carrier)
_, err := js.Publish(ctx, subject, data, nats.Headers(headers))
```

**PostgreSQL (automatic):**
```go
// pgx v5 with otelpgx automatically creates spans for all queries
// when the pool is configured with the OTel tracer.
poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()
```

> [!CAUTION]
> **Reiteration: high-cardinality data is BANNED from OTel metric labels** (see §10 Metric Cardinality Rules). `trace_id`, `span_id`, `user_id`, `tenant_id` must remain as **span attributes** and **log fields** only — never as Prometheus metric labels. Violating this causes OOM in Prometheus and Grafana query timeouts.

---

## 10. Observability — Logging & Metrics

### Logger — Uber Zap

```go
logger := observability.NewLogger(observability.Config{
    Level:   "info",
    Format:  "json",
    Service: "matric-service",
    Env:     "prod",
})
defer observability.Sync(logger)
```

**Structured logging only — never `fmt.Sprintf` inside log calls:**

```go
// ✅ Correct
logger.Infow("message published",
    "trace_id",   envelope.TraceID,
    "intent",     envelope.Intent,
    "latency_ms", elapsed.Milliseconds(),
)

// ❌ Never
logger.Info(fmt.Sprintf("published %s for %s", intent, tenantID))
```

**Always-present log fields:**

| Field | Value |
|---|---|
| `service` | Service name constant |
| `env` | `dev` / `staging` / `prod` |
| `version` | From `VERSION` file |
| `ts` | ISO8601 timestamp |

**Level guidance:**
- `debug` — classifier scores, sequence numbers
- `info` — successful pipeline completions, connections
- `warn` — duplicates, retries, ML unavailability
- `error` / `fatal` — infrastructure failures

### Metrics — Prometheus (RED Method)

Metrics server runs on **`:9090`** separate from the application **`:8080`**.

```go
// Required minimum metrics for every service:
ingress_messages_total         Counter   {channel}              — Rate
ingress_errors_total           Counter   {stage, error_type}    — Errors
normalize_latency_seconds      Histogram {source}               — Duration
classifier_latency_seconds     Histogram {source}               — Duration
dedup_redis_total              Counter   {}                     — Redis dedup hits
dedup_db_total                 Counter   {}                     — DB UNIQUE dedup hits
inflight_messages              Gauge     {}                     — Backpressure
publish_latency_seconds        Histogram {subject}              — NATS publish time
pg_pool_active_conns           Gauge     {}                     — Pool saturation
dlq_records_total              Counter   {reason}               — DLQ entries
```

### Metric Cardinality Rules

> [!CAUTION]
> **High-cardinality data is BANNED from Prometheus metric labels.** Each unique label combination creates a new time series. Unbounded label values cause cardinality explosions that crash Prometheus, exhaust memory, and make dashboards unusable.

| ❌ BANNED as metric labels | ✅ Use instead |
|---|---|
| `user_id`, `tenant_id` | Log field only — query via log aggregator |
| `trace_id`, `request_id` | Log field only — correlate via exemplars |
| `session_id`, `conversation_id` | Log field only |
| `message_id`, `envelope_id` | Log field only |
| Free-form `error.Error()` strings | Map to finite `error_type` enum |

**Rule of thumb:** If a label's cardinality could exceed ~100 unique values, it belongs in **structured logs**, not metric labels.

### Bounded Enums for Metric Labels (Mandatory)

> [!IMPORTANT]
> **All values for metric labels MUST be defined as constants in a dedicated `telemetry` (or `metrics`) package.** Free-form strings passed directly to metric calls cause label drift over time — developers inevitably mix `db_timeout` / `database_down` / `pg_unavailable` for the same error, creating phantom series and broken dashboards.

```go
// internal/observability/telemetry/labels.go

package telemetry

// ──────────────────────────────────────────────────
// Stage labels — used in "stage" label of error/latency metrics.
// Add new stages here; never use raw strings in metric calls.
// ──────────────────────────────────────────────────
const (
    StageIngress        = "ingress"
    StageValidation     = "validation"
    StageClassification = "classification"
    StagePersistence    = "persistence"
    StagePublish        = "publish"
)

// ──────────────────────────────────────────────────
// Error type labels — used in "error_type" label.
// Map every error to ONE of these constants.
// ──────────────────────────────────────────────────
const (
    ErrTypeValidation   = "validation"
    ErrTypeNoIntent     = "no_intent"
    ErrTypeDBTimeout    = "db_timeout"
    ErrTypeDBConstraint = "db_constraint"
    ErrTypeNATSPublish  = "nats_publish"
    ErrTypeMarshal      = "marshal"
    ErrTypeOversized    = "oversized"
    ErrTypeUnknown      = "unknown"
)

// ──────────────────────────────────────────────────
// DLQ reason labels — used in "reason" label of dlq_records_total.
// ──────────────────────────────────────────────────
const (
    DLQReasonTerminalError  = "terminal_error"
    DLQReasonMaxDeliver     = "max_deliver_exceeded"
    DLQReasonOversized      = "oversized_payload"
    DLQReasonBatchFailure   = "batch_persistence_failure"
)
```

**Usage — always reference the constant, never a raw string:**

```go
// ✅ Correct — bounded enum from telemetry package
metrics.IncCounter("ingress_errors_total",
    "stage",      telemetry.StageClassification,  // from enum
    "error_type", telemetry.ErrTypeNoIntent,      // from enum
)

// ❌ BANNED — raw strings cause label drift
metrics.IncCounter("ingress_errors_total",
    "stage", "classification",     // will diverge across PRs
    "error_type", "no_intent",     // someone WILL typo this
)

// ❌ BANNED — unbounded cardinality
metrics.IncCounter("ingress_errors_total",
    "tenant_id", msg.TenantID,     // thousands of tenants
    "error", err.Error(),          // infinite unique strings
)
```

**Enforcement:** Code review must verify that every `IncCounter`, `ObserveLatency`, or `RecordHistogram` call uses a `telemetry.*` constant for every label value. A `go vet` analyzer for this is recommended as a future CI gate.

---

## 11. Error Handling

### Wrapping Convention

Always use `%w` to preserve the error chain. Never swallow errors silently.

```go
// ✅ Correct
return nil, fmt.Errorf("parse postgres dsn: %w", err)

// ❌ Incorrect
return nil, err                            // no wrapping context
return nil, fmt.Errorf("failed: %v", err)  // %v breaks errors.Is/As
```

### Context Propagation

`context.Context` must be the **first argument** of every function that does I/O.

```go
// ✅ Correct
func (r *Repo) Insert(ctx context.Context, record *Record) error { ... }
req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)

// ❌ Never
req, err := http.NewRequest(http.MethodPost, url, body)
```

### Pipeline Error Pattern

```go
func (h *MessageHandler) emitError(ctx context.Context, msg model.RawMessage, errType, code string, err error) {
    traceID, _ := msg.Meta["request_id"].(string)
    envelope := model.ErrorEnvelope{
        TraceID:   traceID,
        TenantID:  msg.TenantID,
        ErrorType: errType,
        ErrorCode: code,
        Message:   err.Error(),
    }
    // Publish to error stream — best-effort, non-blocking
    go func() {
        if pubErr := h.publisher.PublishJSON(ctx, "evt.error."+errType+".v1", envelope); pubErr != nil {
            h.logger.Warnw("error event publish failed", "error", pubErr)
        }
    }()
    h.logger.Errorw("pipeline error",
        "trace_id", traceID,
        "type",     errType,
        "code",     code,
        "error",    err,
    )
}
```

### Infrastructure Init Failures

```go
if err := pool.Ping(ctx); err != nil {
    logger.Fatalw("postgres unreachable", "error", err)
    // Fatal flushes logs and calls os.Exit(1)
    // A service that cannot reach its dependencies must not start
}
```

---

## 12. Build, Docker & Kubernetes

### Local Development

```bash
cp .env.sample .env   # edit with local values — never commit this file

docker compose up -d nats redis postgres

# Apply migrations (golang-migrate)
migrate -path migrations/ -database "$DATABASE_URL" up

make build && make run

make lint
make test
```

### Docker Compose

```yaml
services:
  nats:
    image: nats:2.10-alpine
    command: "-js"
    ports: ["4222:4222"]
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER:     local_user
      POSTGRES_PASSWORD: local_pass
      POSTGRES_DB:       local_db
    ports: ["5432:5432"]
```

### Dockerfile — Multi-Stage

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=$(cat VERSION)" \
    -o /bin/service \
    ./cmd/matric-service/main.go

FROM gcr.io/distroless/static-debian12
COPY --from=builder /bin/service /service
EXPOSE 8080 9090
ENTRYPOINT ["/service"]
```

### Makefile

```makefile
SERVICE   := matric-service
REGISTRY  := anomaly_detection_platform
VERSION   := $(shell cat VERSION)

.PHONY: build test lint run docker

build:
	go mod tidy
	go build -o bin/$(SERVICE) ./cmd/$(SERVICE)/main.go

test:
	go test ./... -v -race -count=1

lint:
	golangci-lint run ./...

run:
	go run ./cmd/$(SERVICE)/main.go

docker:
	docker build -t $(REGISTRY)/$(SERVICE):$(VERSION) \
	             -t $(REGISTRY)/$(SERVICE):latest .

bump-patch:
	@awk -F. '{print $$1"."$$2"."$$3+1}' VERSION > VERSION.tmp && mv VERSION.tmp VERSION

bump-minor:
	@awk -F. '{print $$1"."$$2+1".0"}' VERSION > VERSION.tmp && mv VERSION.tmp VERSION

bump-major:
	@awk -F. '{print $$1+1".0.0"}' VERSION > VERSION.tmp && mv VERSION.tmp VERSION
```

### Kubernetes Manifests

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: matric-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: matric-service
  template:
    metadata:
      labels:
        app: matric-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port:   "9090"
        prometheus.io/path:   "/metrics"
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: matric-service
          image: <ECR_URL>/matric-service:<GIT_SHA>
          ports:
            - containerPort: 8080
            - containerPort: 9090
          envFrom:
            - secretRef:    { name: matric-service-secrets }
            - configMapRef: { name: matric-service-config }
          livenessProbe:
            httpGet: { path: /admin/healthz, port: 8080 }
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet: { path: /admin/readyz, port: 8080 }
            initialDelaySeconds: 3
            periodSeconds: 5
          resources:
            requests: { cpu: "100m", memory: "128Mi" }
            limits:   { cpu: "500m", memory: "512Mi" }
```

```yaml
# secret.yaml — never commit values
apiVersion: v1
kind: Secret
metadata:
  name: matric-service-secrets
type: Opaque
stringData:
  DATABASE_URL: ""
  REDIS_ADDR: ""
  REDIS_PASSWORD: ""
  NATS_URL: ""
```

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: matric-service-config
data:
  LOG_LEVEL:    "info"
  LOG_FORMAT:   "json"
  ENV:          "prod"
  HTTP_ADDR:    ":8080"
  METRICS_ADDR: ":9090"
  BATCH_SIZE:   "100"
  FLUSH_EVERY:  "2s"
  CONCURRENCY:  "4"
  DEDUPE_TTL:   "5m"
```

---

## 13. Code Quality & Linting

### `.golangci.yml`

```yaml
linters:
  enable:
    - errcheck          # all error returns must be checked
    - wrapcheck         # errors must be wrapped with %w
    - contextcheck      # context.Context must be propagated
    - noctx             # http.NewRequest without context is forbidden
    - govet             # standard compiler checks
    - staticcheck       # bug detection and dead code
    - bodyclose         # HTTP response bodies must be closed
    - gofmt             # consistent formatting
    - goimports         # import grouping and ordering
    - unused            # unexported unused symbols
    - gochecknoinits    # init() functions are BANNED
    - depguard          # enforce import restrictions

linters-settings:
  wrapcheck:
    ignoreSigs:
      - .Errorf(
  depguard:
    rules:
      domain-no-infra:
        deny:
          - pkg: "*/internal/infra"
            desc: "Domain/app layer must not import infrastructure adapters"
        files:
          - "**/internal/domain/**"
          - "**/internal/app/**"
          - "**/internal/classify/**"
          - "**/pkg/normalizer/**"

issues:
  exclude-rules:
    - path: "_test.go"
      linters: [wrapcheck, errcheck]
```

Run: `make lint` — blocks CI if any linter fails. **Zero tolerance.**

### Security Standards

| Standard | Rule |
|---|---|
| Credential masking | All DSNs through `sanitizeDSN` before logging |
| No secrets in code | `.env.sample` has placeholder values only |
| Container security | Final image: `distroless/static` or `scratch` |
| Secrets management | Kubernetes `Secret` for credentials, never `ConfigMap` |
| Leak prevention | `gitleaks` scan in CI before Docker build |
| **Crypto-secure randomness** | **`math/rand` is BANNED** for critical logic (see below) |

### Cryptographically Secure Randomness

> [!CAUTION]
> **`math/rand` is BANNED for any critical logic** — sequencer offsets, backoff jitter, ID generation, token creation. In Kubernetes environments, pods starting simultaneously with the same `time.Now()` seed produce identical sequences from `math/rand`, causing:
> - Identical sequence offsets across replicas → predictable counters
> - Identical backoff jitter → thundering herd on retry
> - Duplicate "random" IDs → constraint violations

Use `crypto/rand` everywhere:

```go
import "crypto/rand"
import "encoding/binary"

// cryptoRandInt returns a cryptographically secure random int in [0, max)
func cryptoRandInt(max int) int {
    var b [8]byte
    _, _ = rand.Read(b[:])
    return int(binary.BigEndian.Uint64(b[:]) % uint64(max))
}

// ✅ Correct — crypto/rand
offset := int64(cryptoRandInt(10000) + 1000)
jitter := time.Duration(cryptoRandInt(500)) * time.Millisecond

// ❌ BANNED — math/rand
offset := int64(rand.Intn(10000) + 1000)   // deterministic seed collision!
```

---

## 14. Testing Strategy

### Unit Tests — Table-Driven

```go
func TestComputeDedupeKey(t *testing.T) {
    tests := []struct {
        name string
        msg  model.RawMessage
        want string
    }{
        {
            name: "stable key from canonical fields",
            msg: model.RawMessage{
                TenantID:       "t1",
                ClientID:       "c1",
                Channel:        "slack",
                SessionID:      "s1",
                ConversationID: "conv1",
                EnvelopeID:     "env1",
            },
            want: "expected-sha256-hex",
        },
        {
            name: "different rawText same IDs produces same key",
            msg: model.RawMessage{
                TenantID:       "t1",
                ClientID:       "c1",
                Channel:        "slack",
                SessionID:      "s1",
                ConversationID: "conv1",
                EnvelopeID:     "env1",
                RawText:        "completely different text",  // must NOT affect key
            },
            want: "expected-sha256-hex",  // same as above
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := computeDedupeKey(tt.msg)
            if got != tt.want {
                t.Errorf("got %s, want %s", got, tt.want)
            }
        })
    }
}
```

### Interface-Based Mocking (Ports & Adapters)

Because the domain defines interfaces (ports), all business logic tests use in-memory stubs — no Docker, no network.

```go
// Test stub — implements domain.Deduplicator
type mockDedup struct{ seen map[string]bool }

func (m *mockDedup) Check(_ context.Context, key string) (bool, error) {
    if m.seen[key] { return false, nil }
    m.seen[key] = true
    return true, nil
}

func (m *mockDedup) Reset(_ context.Context, key string) error {
    delete(m.seen, key)
    return nil
}
```

### Integration Tests

```go
func TestMain(m *testing.M) {
    pool := setupTestPostgres()
    applyMigrations(pool)
    code := m.Run()
    cleanupDB(pool)
    os.Exit(code)
}
```

**What to test:**
- Each intent handler: happy path + parse error + validation failure
- `store.Writer`: records appear in Postgres after flush; backpressure blocks when buffer is full
- `Deduplicator`: new vs duplicate key via real/mock Redis
- `Pipeline`: inject `RawMessage` → assert `CommandEnvelope` published
- **DLQ semantics**: DB `UNIQUE` violation → Ack + metric, NOT DLQ entry
- **Shutdown**: cancel context → writer flushes remaining records → pool closes after writer stops

---

## 15. Git, Versioning & CI/CD

### Semantic Versioning

```bash
cat VERSION        # e.g. 1.4.2
make bump-patch    # 1.4.2 → 1.4.3
make bump-minor    # 1.4.2 → 1.5.0
make bump-major    # 1.4.2 → 2.0.0
```

Version is injected at build time: `-ldflags "-X main.version=$(cat VERSION)"`. Exposed via `GET /admin/version`.

### CI/CD Pipeline (GitHub Actions → AWS ECR)

```
push to main
  └── Checkout
  └── Setup Go 1.24
  └── make lint         ← blocks on lint failure (including gochecknoinits)
  └── make test         ← blocks on test failure
  └── gitleaks detect   ← blocks on secrets detected
  └── Configure AWS CLI
  └── Login to ECR
  └── docker build -t <ECR>/<service>:latest
                  -t <ECR>/<service>:<short-git-sha>
  └── docker push both tags
```

**Always use the `:<git-sha>` tag in Kubernetes** for deterministic, rollback-capable deployments.

### Migration Strategy

- Migrations are **forward-only**: never edit an applied file
- Fix mistakes with a **new** migration
- All DDL statements must be idempotent (`IF NOT EXISTS`)
- Use [golang-migrate](https://github.com/golang-migrate/migrate) for tracking via `schema_migrations` table

---

## 16. SLOs, Alerts & Operational Readiness

Writing code is half the job. Operating a healthy service requires defined SLOs and alerts that fire before users notice problems.

### Baseline SLOs

| SLO | Target | Measurement |
|---|---|---|
| **Normalize latency (p99)** | ≤ 200ms | `histogram_quantile(0.99, normalize_latency_seconds)` |
| **Classify latency (p99)** | ≤ 500ms | `histogram_quantile(0.99, classifier_latency_seconds)` |
| **NATS publish latency (p99)** | ≤ 50ms | `histogram_quantile(0.99, publish_latency_seconds)` |
| **Error rate** | < 0.1% of ingested messages | `ingress_errors_total / ingress_messages_total` |
| **Consumer lag** | < 1,000 messages behind head | NATS `consumer_pending` metric |
| **DLQ rate** | < 5 records / hour | `rate(dlq_records_total[1h])` |
| **Availability** | 99.9% (8h 45m/year downtime) | Uptime of `/admin/healthz` |

### Mandatory Alerts

Every service **must** have these alerts configured before production deployment.

| Alert Name | Condition | Severity | Action |
|---|---|---|---|
| `HighNormalizeLatency` | p99 > 500ms for 5 min | Warning | Check classifier service, DB pool |
| `HighErrorRate` | error rate > 1% for 5 min | Critical | Page on-call, check logs |
| `ConsumerLagHigh` | pending > 5,000 for 10 min | Warning | Scale consumers, check pipeline bottleneck |
| `DLQRateHigh` | > 10 records/hour | Warning | Inspect DLQ records, check Postgres health |
| `PGPoolExhausted` | active conns ≥ max conns for 5 min | Critical | Increase `PG_MAX_CONNS` or optimize queries |
| `RedisConnectionError` | redis connection failures > 0 for 2 min | Critical | Check Redis cluster, network |
| `NATSDisconnected` | NATS client disconnected for > 30s | Critical | Check NATS broker health |
| `PodRestartLoop` | > 3 restarts in 15 min | Critical | Check `kubectl logs`, OOMKilled, panic |
| `InFlightBackpressure` | `inflight_messages` at max for 5 min | Warning | Pipeline stalled — check Postgres/NATS |

### Prometheus Rules Example

```yaml
groups:
  - name: matric-service
    rules:
      - alert: HighNormalizeLatency
        expr: histogram_quantile(0.99, rate(normalize_latency_seconds_bucket[5m])) > 0.5
        for: 5m
        labels: { severity: warning }
        annotations:
          summary: "p99 normalization latency exceeds 500ms"

      - alert: DLQRateHigh
        expr: rate(dlq_records_total[1h]) > 10
        for: 5m
        labels: { severity: warning }
        annotations:
          summary: "DLQ is accumulating records — check Postgres health"

      - alert: PGPoolExhausted
        expr: pg_pool_active_conns >= pg_pool_max_conns
        for: 5m
        labels: { severity: critical }
        annotations:
          summary: "PostgreSQL connection pool is saturated"
```

### Operational Readiness Checklist

Before any service is deployed to production, the following must be verified:

- [ ] All SLOs from the table above have corresponding Grafana panels
- [ ] All mandatory alerts are configured in Prometheus/Alertmanager
- [ ] Runbook exists for each Critical alert with step-by-step remediation
- [ ] DLQ has a scheduled daily inspection by the team
- [ ] `terminationGracePeriodSeconds` is set to at least 30s
- [ ] Load test confirms p99 stays under SLO at 2× expected peak traffic

---

## 17. Decision Log

This section records the key architectural decisions made in this document, their rationale, and what they replace.

| # | Decision | Rationale | Replaces |
|---|---|---|---|
| ADR-1 | **Callback Ack pattern** — `BatchItem` carries `OnAck`/`OnNak` closures; writer calls `OnAck` only after `tx.Commit` | Zero data loss; Ack is impossible until Postgres confirms persistence | `m.Ack()` immediately after `Enqueue` |
| ADR-2 | **Unified context tree** — all goroutines derive from signal context | Prevents zombie goroutines; ensures writer drains on SIGTERM | `context.WithCancel(context.Background())` in Writer |
| ADR-3 | **Strict Ports & Adapters** — domain defines interfaces, infra implements | Testable business logic without Docker; swappable adapters | Domain importing `internal/infra` |
| ADR-4 | **DB UNIQUE = success** — duplicate on dedupe_key is Ack + metric, not DLQ | DLQ reserved for truly broken messages; duplicates are expected behavior | All constraint violations → DLQ |
| ADR-5 | **PII-safe dedup keys** — only canonical IDs in hash input | Prevents PII leak in Redis; ensures hash stability across encodings | SHA256 of `rawText` + 5 fields |
| ADR-6 | **Ban `init()` globally** — `gochecknoinits` linter enforced | Explicit wiring is auditable and testable; `init()` is implicit and order-dependent | Handler self-registration via `init()` |
| ADR-7 | **RequestID before Logger** — middleware order enforced | Every log line carries `trace_id`; out-of-order produces blind spots | Logger before RequestID |
| ADR-8 | **Minimal 202 response** — `{"status":"accepted","request_id":"..."}` only | Prevents PII/payload leaks in client logs and monitoring | Echoing full request body |
| ADR-9 | **Cardinality ban** — no `tenant_id`, `trace_id`, etc. in metric labels | Prevents Prometheus OOM and dashboard degradation | `tenant` as a Prometheus label |
| ADR-10 | **SLOs & Alerts required** — baseline targets with mandatory Prometheus rules | Shifts engineering from "ship code" to "operate a healthy service" | No operational requirements |
| ADR-11 | **`net/http` + `chi` PRIMARY, Fiber acceptable** — dual-framework policy | Native `context.Context` for OTel/gRPC; Fiber allowed with `UserContext()` bridge | Fiber-only (or chi-only) |
| ADR-12 | **OTel native integration** — `traceparent` extraction at every ingress | End-to-end distributed tracing through HTTP, NATS, Postgres | Manual `X-Request-ID` correlation only |
| ADR-13 | **HTTP rate limiting mandatory** — `httprate` per-IP and per-tenant | Prevents DDoS, accidental spam, and OOM from upstream floods | No ingress rate limiting |
| ADR-14 | **`http.MaxBytesReader` mandatory** — payload size cap on all ingress | Prevents OOM from oversized payloads; bounded channel count is not enough | Unbounded request body size |
| ADR-15 | **`inflight_bytes` gauge** — tracks actual memory footprint of writer buffer | Item count alone hides payload-size variance; enables memory-based alerts | `inflight_messages` count gauge only |
| ADR-16 | **`crypto/rand` mandatory** — `math/rand` banned for critical logic | Deterministic seed collisions in K8s cause duplicate offsets, IDs, and jitter | `math/rand.Intn()` for sequencer offsets |
| ADR-17 | **Transactional Outbox** — future target for atomic event publishing | Eliminates dual-write anomaly between Postgres and NATS | Direct NATS publish after DB write |
| ADR-18 | **Explicit variable shadowing in closures** — `msg := m` before `OnAck`/`OnNak` | Code review clarity and developer peace of mind; safe even before Go 1.24 | Implicit loop variable capture |
| ADR-19 | **Strict DLQ semantics** — transient errors → Nak only; terminal errors → Ack + DLQ | Prevents DLQ pollution with retryable messages; stops poison pill redelivery loops | All errors treated uniformly |
| ADR-20 | **Hard NATS payload size limit** — `len(msg.Data()) > maxPayloadSize` → Ack + DLQ | Defence-in-depth against OOM in writer buffer; HTTP `MaxBytesReader` only covers ingress | No size guard on NATS consumer |
| ADR-21 | **Bounded enum constants for metric labels** — `telemetry.*` constants mandatory | Prevents cardinality drift and label typos over time; enforces finite series space | Free-form strings in metric calls |

---

## 18. Future Architecture — Transactional Outbox

> [!NOTE]
> This section describes the **target architecture** for critical event publishing. It is not yet implemented but is the intended evolution of the current "Ack after DB flush" pattern.

### The Dual-Write Problem

The current architecture writes to PostgreSQL first, then publishes to NATS. If the process crashes between the successful `tx.Commit()` and the NATS `Publish`, the event is lost — the database has the record, but no downstream consumer is notified. The callback Ack pattern (ADR-1) mitigates this for inbound messages but does not solve the forward-publish problem.

### Transactional Outbox Pattern

The **Transactional Outbox** eliminates this dual-write anomaly by writing both the domain state change and the outbound event in a **single PostgreSQL transaction**.

```
┌─────────────────────────────────────────────────┐
│                Single Transaction                │
│                                                  │
│  INSERT INTO ingress.normalized (...) VALUES ... │
│  INSERT INTO ingress.outbox (subject, payload,   │
│              created_at, published) VALUES ...    │
│                                                  │
│  COMMIT                                          │
└────────────────────┬────────────────────────────┘
                     │
           ┌─────────▼──────────┐
           │  Outbox Relay      │   (background goroutine or separate sidecar)
           │                    │
           │  SELECT FROM outbox│
           │  WHERE NOT published│
           │  ORDER BY id       │
           │                    │
           │  → Publish to NATS │
           │  → UPDATE outbox   │
           │    SET published=t │
           └────────────────────┘
```

```sql
-- migrations/NNNN_create_outbox.sql
CREATE TABLE IF NOT EXISTS ingress.outbox (
    id         BIGSERIAL PRIMARY KEY,
    subject    TEXT NOT NULL,       -- NATS subject (e.g. "evt.normalized.v1")
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_outbox_unpublished
    ON ingress.outbox (id) WHERE NOT published;
```

**Key properties:**
- **Atomicity** — domain state and event are committed in one transaction. No message can be lost between steps.
- **At-least-once delivery** — the relay may publish the same event twice (crash after publish, before marking `published`). Downstream consumers still rely on idempotency (§8).
- **Ordering** — events are relayed in `id` order, preserving causal ordering within a partition.

**When to adopt:** When the platform reaches the point where forward-publish loss is unacceptable (e.g., trade execution events, settlement commands). Until then, the callback Ack pattern with 3-layer idempotency provides a strong guarantee for the inbound path.

---

## 19. Quick-Start Checklist

Use this as a **PR gate** for every new service repository.

**Project Setup**
- [ ] Directory structure follows `cmd/` / `internal/` / `pkg/` / `migrations/` layout
- [ ] `internal/domain/ports.go` defines all interfaces — domain never imports `internal/infra`
- [ ] `pkg/model/` has zero internal imports
- [ ] `VERSION` file with semver; `.env.sample` has placeholder values only
- [ ] `.env` is in `.gitignore`

**Wiring & Startup**
- [ ] All dependencies wired manually in `cmd/<service>/main.go` — no `init()` magic
- [ ] `gochecknoinits` linter enabled — blocks CI on `init()` functions
- [ ] Handler registration is explicit in main.go
- [ ] Logger initialized first, `defer observability.Sync(log)` in main
- [ ] Config loaded from environment via `config.Load()`
- [ ] Fatal on infra init failure (Postgres, Redis, NATS)
- [ ] Single root `ctx` from `signal.NotifyContext` — all goroutines derive from it
- [ ] Shutdown order: `publisher → pgWriter.Stop() → redis → pgPool → nats.Drain()`

**HTTP (net/http + chi PRIMARY / Fiber acceptable)**
- [ ] `chi` router with `middleware.RequestID` registered BEFORE `otelhttp` BEFORE `StructuredLogger`
- [ ] `otelhttp.NewMiddleware()` in middleware stack — `traceparent` extraction
- [ ] `httprate` rate limiting on all ingress routes (per-IP or per-tenant)
- [ ] `http.MaxBytesReader` on all ingress handlers — payload size cap
- [ ] All ingress endpoints return `{"status":"accepted","request_id":"..."}` — never echo body
- [ ] If using Fiber: `c.UserContext()` bridge in every handler
- [ ] `/admin/healthz`, `/admin/readyz` endpoints implemented
- [ ] Metrics exposed on separate port (`:9090`)

**NATS**
- [ ] Subjects follow `<type>.<domain>.<action>.v<version>` convention
- [ ] Durable consumers with `AckExplicitPolicy`
- [ ] `BatchItem` with `OnAck`/`OnNak` closures — **consumer never calls `m.Ack()` directly**
- [ ] Explicit variable shadowing (`msg := m`) before closures in consumer loop
- [ ] Hard payload size check (`len(msg.Data()) > maxPayloadSize`) at top of consumer loop — oversized → Ack + DLQ + `dropped_oversized_total` metric
- [ ] Strict DLQ semantics: transient errors → `Nak()` only; terminal errors → `Ack()` + DLQ write
- [ ] `PublishJSON` for all commands (3-retry with linear backoff)
- [ ] `traceparent` injected into NATS headers on publish, extracted on consume
- [ ] Context always propagated — never `context.Background()` in inflight work

**Database**
- [ ] pgx pool with `application_name` in DSN
- [ ] Async `store.Writer` with **callback Ack pattern** and **backpressure** — never drops records
- [ ] `inflight_bytes` Prometheus gauge tracks buffer memory footprint
- [ ] `pgWriter.Stop()` called before `pgPool.Close()`
- [ ] `ON CONFLICT (dedupe_key) DO NOTHING` — duplicates are Ack, not DLQ
- [ ] DLQ reserved for unprocessable/malformed messages only
- [ ] All SQL uses `$1`, `$2` parameterized placeholders
- [ ] All migrations use `IF NOT EXISTS`; managed by `golang-migrate`

**Redis**
- [ ] Dedup keys use canonical IDs only — NO `rawText` or PII
- [ ] SETNX dedup gate as first pipeline step
- [ ] All keys (except classifier rules) have TTL
- [ ] `SCAN` in tooling — never `KEYS *`

**Idempotency & Tracing**
- [ ] 3-layer idempotency: Redis SETNX + NATS AckExplicit + DB UNIQUE constraint
- [ ] `CommandEnvelope.TraceID` populated in every `ProcessRaw` call — never empty
- [ ] `trace_id` included in all significant log statements

**Observability**
- [ ] Zap SugaredLogger with structured key-value fields only
- [ ] RED metrics: Rate (counter), Errors (counter), Duration (histogram)
- [ ] **No high-cardinality labels** — `tenant_id`, `trace_id`, `user_id` banned from metrics
- [ ] **Bounded enum constants** — all metric label values use `telemetry.*` constants, never raw strings
- [ ] `sanitizeDSN` applied before logging any database URL

**SLOs & Alerts**
- [ ] All baseline SLOs from §16 have Grafana panels
- [ ] All mandatory alerts from §16 configured in Prometheus
- [ ] Runbooks exist for every Critical alert

**Security & Build**
- [ ] Multi-stage Dockerfile: `golang:1.24-alpine` → `distroless/static-debian12`
- [ ] `CGO_ENABLED=0 GOOS=linux` in Docker build
- [ ] `VERSION` injected via `-ldflags`
- [ ] **`crypto/rand` only** — `math/rand` banned for sequencer offsets, jitter, IDs
- [ ] Kubernetes `Secret` for credentials, `ConfigMap` for tuning
- [ ] `terminationGracePeriodSeconds: 30` on pod spec
- [ ] Liveness → `/admin/healthz`, readiness → `/admin/readyz`

**Quality**
- [ ] `.golangci.yml` committed with `wrapcheck`, `contextcheck`, `noctx`, `gochecknoinits`, `depguard`
- [ ] `gitleaks` scan in CI pipeline
- [ ] `make lint` passes with zero errors
- [ ] Table-driven unit tests for all intent handlers
- [ ] Integration tests for all infra adapters
- [ ] CI pipeline: `lint → test → gitleaks → docker build → push`