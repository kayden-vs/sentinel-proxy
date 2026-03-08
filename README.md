# Sentinel Proxy

**Real-time streaming data exfiltration detection and prevention proxy.**

Built for **DoubleSlash 4.0** — Cyber Security Track.

---

## Demo Video

https://drive.google.com/file/d/1sCVHzwE7AitDCNgoWd-4Y77v5Duwj92k/view?usp=sharing

---

## The Problem

An insider or attacker with valid credentials opens a data API and starts streaming out your entire database. Traditional security tools (WAFs, DLP agents) inspect request/response payloads after the fact — by then, the data is already gone. There is no widely available, lightweight solution that can monitor and kill a data stream *mid-transfer* based on behavioral anomalies.

## Our Solution

Sentinel Proxy is a reverse proxy that sits between your application and its backend services. It monitors every chunk of data flowing through in real time, learns what "normal" looks like per user, and kills the stream the moment behavior deviates — all in under 500ms with zero human intervention.

No ML models. No external SaaS dependencies. Just ~2,800 lines of Go.

---

## How It Works

```
Client  ──HTTP──▶  Sentinel Proxy  ──gRPC stream──▶  Backend
                       │
                 ┌─────┴─────┐
                 │  Per-chunk │
                 │  analysis  │
                 └─────┬─────┘
                       │
            ┌──────────┼──────────┐
            ▼          ▼          ▼
        Identity   Threshold   Policy
        Resolver    Engine    Enforcer
            │          │          │
            └──────────┼──────────┘
                       │
                    Redis
              (behavior store)
```

1. **Identity Resolution** — Every request is attributed to a user through a 3-layer fallback: JWT token → API key → IP/User-Agent fingerprint. Stripping credentials doesn't help; the proxy falls back to fingerprinting with the strictest limits.

2. **Behavioral Baseline (Redis)** — Each user's historical transfer volumes are tracked using Redis sorted sets. A Lua script computes a blended average of windowed (recent) and lifetime behavior in a single round-trip.

3. **Adaptive Threshold Engine** — Per-request byte limits are computed dynamically:
   - **Global floor** — minimum allowed for any user (default 1MB).
   - **Adaptive threshold** — `historical_avg × burst_multiplier × role_multiplier`.
   - **Absolute ceiling** — hard cap that cannot be exceeded (default 5MB for standard users).
   - New users with insufficient history get the floor.

4. **Real-time Stream Monitoring** — Every gRPC chunk is inspected as it passes through. The monitor computes current transfer rate, checks for rate anomalies (current rate significantly exceeding historical average), and applies adaptive throttle delays when approaching limits.

5. **Graduated Enforcement (Grace System)** — Soft breaches (adaptive threshold exceeded) trigger a graduated response tracked per-user in Redis:
   - 1st violation → **Log only** (could be a legitimate spike)
   - 2nd violation → **Throttle** (slow down the stream)
   - 3rd violation → **Terminate** (kill the connection)

   Hard breaches (absolute ceiling exceeded) bypass grace entirely and terminate immediately.

6. **Fail-Open Design** — If Redis goes down, the proxy continues operating with safe default thresholds instead of blocking all traffic. Availability is never sacrificed.

---

## Key Features

- **Sub-second kill time** — Exfiltration attempts are detected and terminated mid-stream, typically in under 500ms.
- **Per-user behavioral learning** — Thresholds adapt based on each user's actual usage patterns, not static rules.
- **Role-based and endpoint-based policies** — Admins, analysts, and exporters get different multipliers. Specific endpoints (e.g., `/export`) can have custom limits.
- **3-layer identity fallback** — JWT → API Key → IP fingerprint. No anonymous escape hatch.
- **Rate anomaly detection** — Flags streams where the current byte rate is 5x+ the user's historical average.
- **Adaptive throttling** — Throttle delay scales proportionally with how far over threshold a stream is.
- **Prometheus metrics** — Every decision, kill, violation, byte count, and stream duration is instrumented and exportable.
- **gRPC streaming backend** — Backend uses server-side streaming for realistic, chunk-by-chunk data delivery.
- **Configurable via YAML** — All thresholds, timeouts, policies, and feature flags in a single config file.

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.25 |
| Proxy ↔ Backend | gRPC with server-side streaming (protobuf) |
| Behavior Store | Redis (sorted sets + Lua scripting) |
| Identity | JWT (HMAC-SHA256), API keys, IP fingerprinting |
| Metrics | Prometheus client library |
| Configuration | YAML |

---

## Project Structure

```
sentinel-proxy/
├── cmd/
│   ├── proxy/main.go          # HTTP reverse proxy with enforcement
│   └── backend/main.go        # gRPC streaming backend (simulates data service)
├── internal/
│   ├── config/                 # YAML configuration loader with defaults
│   ├── identity/               # 3-layer identity resolver (JWT, API key, fingerprint)
│   ├── metrics/                # Prometheus metrics registry
│   ├── policy/                 # Graduated enforcement engine (grace system)
│   ├── redis/                  # Redis client with fail-open wrapper + Lua scripts
│   ├── stream/                 # Real-time stream monitor (per-chunk analysis)
│   └── threshold/              # Adaptive threshold computation engine
├── proto/sentinel/             # Protobuf definitions and generated code
├── config/sentinel.yaml        # Default configuration
└── scripts/
    ├── start.sh                # Start all services locally
    ├── stop.sh                 # Stop services
    ├── demo.sh                 # Interactive step-by-step demo
    └── logwatch.sh             # Live log feed for demos
```

---

## Getting Started

### Prerequisites

- Go 1.25+
- Redis 7+ (optional — system works without it in fail-open mode)
- `protoc` with Go plugins (only if modifying `.proto` files)

### Build

```bash
make build
```

This produces two binaries in `bin/`:
- `sentinel-proxy` — the HTTP reverse proxy
- `sentinel-backend` — the gRPC data service backend

### Run

**Quick start (both services):**

```bash
./scripts/start.sh
```

**Or manually:**

```bash
# Terminal 1: Start the gRPC backend
make run-backend

# Terminal 2: Start the proxy
make run-proxy
```

The proxy listens on `:8080`, backend on `:9090`, and metrics on `:9100`.

### Run the Demo

The interactive demo walks through 5 scenarios with a step-by-step presentation flow:

```bash
# Terminal 1 (main): Interactive demo
./scripts/demo.sh

# Terminal 2 (side): Live log feed
./scripts/logwatch.sh
```

**Demo scenarios:**

| Act | Scenario | What happens |
|-----|----------|--------------|
| 1 | Normal user request | Stream completes normally, baseline recorded |
| 2 | Data exfiltration attack | Stream killed mid-transfer at ceiling (~5MB out of 60-100MB) |
| 3 | Repeated attacker | Graduated enforcement: log → throttle → terminate |
| 4 | Anonymous attack (no credentials) | IP fingerprint fallback still catches and kills |
| 5 | Observability | Live Prometheus metrics from all previous scenarios |

### Run Tests

```bash
make test
```

---

## Configuration

All settings are in [`config/sentinel.yaml`](config/sentinel.yaml). Key sections:

**Thresholds:**
```yaml
threshold:
  global_floor_bytes: 1048576       # 1MB minimum for any user
  burst_multiplier: 3.0             # 3x historical average allowed
  absolute_ceiling_bytes: 5242880   # 5MB hard cap
  rate_anomaly_factor: 5.0          # 5x rate = anomaly flag
```

**Role multipliers:**
```yaml
policies:
  role_multipliers:
    admin: 5.0
    analyst: 3.0
    exporter: 10.0
    user: 1.0
```

**Grace violations (graduated enforcement):**
```yaml
policies:
  grace_violations:
    violation_window_sec: 300       # 5-minute sliding window
    log_only_count: 1               # 1st offense: log
    throttle_count: 2               # 2nd offense: throttle
    terminate_count: 3              # 3rd offense: kill
```

---

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /data` | Standard data stream (enforced) |
| `GET /export` | Export data stream (elevated limits) |
| `GET /health` | Health check with Redis status |
| `GET /simulate/normal` | Simulate normal user traffic |
| `GET /simulate/attack` | Simulate exfiltration attack (~60-100MB) |
| `GET /simulate/export` | Simulate large export |
| `GET :9100/metrics` | Prometheus metrics |

---

## Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `sentinel_proxy_bytes_streamed_total` | Counter | Total bytes streamed per user/endpoint |
| `sentinel_proxy_requests_total` | Counter | Total requests by endpoint/status/identity method |
| `sentinel_proxy_stream_kills_total` | Counter | Stream terminations by reason/endpoint |
| `sentinel_proxy_anomalies_detected_total` | Counter | Rate anomalies detected |
| `sentinel_proxy_active_streams` | Gauge | Currently active streaming connections |
| `sentinel_proxy_stream_duration_seconds` | Histogram | Stream duration distribution |
| `sentinel_proxy_bytes_per_request` | Histogram | Bytes-per-request distribution |
| `sentinel_proxy_violations_by_grade_total` | Counter | Violations by enforcement grade |

---

## Team

Built by **Cloud9** at DoubleSlash 4.0 Hackathon.
