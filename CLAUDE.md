# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HMDP-Go is a Go port of the "Heima Dianping" O2O social commerce platform — a review/social app covering user auth, shop discovery, blogging, voucher/coupon management, and high-concurrency flash sales (seckill). It's a monolithic Go application using Gin + GORM + Redis + Kafka.

**Module:** `github.com/amemiya02/hmdp-go` | **Go:** 1.25.6

## Commands

```bash
# Run the server (default :8081)
go run ./cmd/api/main.go

# Install dependencies
go mod download

# Fast tests (no external infrastructure)
go test -race -count=1 ./...

# Integration tests (requires isolated MySQL and Redis)
go test -p=1 -v -count=1 -tags=integration \
  ./internal/util/ ./internal/service/seckill/ ./internal/service/order/ ./internal/service/

# Load tests are opt-in
go test -v -count=1 -tags=load -run '^TestLoadTest' ./internal/service/

# Generate test report
./scripts/gen-test-report.sh
```

## Architecture

**Layered design:** Handler → Service → Repository, with manual constructor injection (no DI framework).

- `cmd/api/main.go` — Entry point. Explicitly initializes infrastructure, starts the Kafka consumer, and waits for graceful shutdown.
- `internal/handler/` — HTTP handlers. Parse requests, bind params, return JSON via `dto.Result`.
- `internal/service/` — Business logic. Orchestrates repos, Redis, caching.
- `internal/repository/` — Pure GORM queries. Stateless structs.
- `internal/model/entity/` — GORM models mapping to `tb_*` tables.
- `internal/model/dto/` — DTOs: `Result` (API response wrapper), `UserDTO`, `LoginForm`, `ScrollResult`.
- `internal/global/` — Existing singleton clients, explicitly opened by `global.Init(ctx)` and closed by `global.Close()`; importing the package has no network side effects.
- `internal/constant/` — Redis key prefixes/TTLs, system constants, regex patterns.
- `internal/util/` — Cache strategies, distributed locks, ID generator, user context helper.
- `internal/middleware/` — Two-layer auth: global `RefreshTokenInterceptor` + route-level `LoginInterceptor`.
- `internal/service/seckill/` — Redis+Lua atomic pre-check + rollback for flash sales.
- `internal/service/order/` — Strategy pattern: `Executor` interface with Sync (V1), Channel (V2), Kafka (V3) implementations. V3 is the default HTTP route and a production-hardening prototype.

## Configuration

Viper loads `config/config.yaml` at package init time. Config path resolved via `runtime.Caller(0)` so tests work too. Sub-structs: `ServerConfig`, `MySQLConfig`, `RedisConfig`, `KafkaConfig`.

## Seckill Flow (Key Feature)

Three evolutionary versions of the same seckill operation, all sharing the same Redis Lua pre-check:

1. **V1 (Sync):** Lua pre-check → distributed lock (Redisson-style with watchdog) → DB transaction
2. **V2 (Channel prototype):** Lua pre-check → buffered Go channel; no production persistence consumer is wired
3. **V3 (default route):** Lua pre-check + Redis pending record → synchronous all-ISR Kafka ACK or background relay → DB transaction → manual offset commit

The `seckill.lua` script atomically checks stock > 0, checks user not in Set, deducts stock, adds user to Set. Returns 0/1/2 for success/stock-empty/duplicate.

## Cache Strategies (`internal/util/cache.go`)

- `QueryWithMutex` — Cache-aside + null-value caching and distributed mutex rebuild

## Distributed Locks (`internal/util/`)

- `SimpleRedisLock` — SETNX + UUID token + Lua compare-and-delete unlock
- `RedissonLock` — Watchdog goroutine (TTL renewal at 1/3 interval) + spin-retry with wait timeout

## Redis Data Structures

| Feature | Type | Key Pattern |
|---|---|---|
| Login session | Hash | `login:user:{token}` |
| Shop cache | String (JSON) | `cache:shop:{id}` |
| Seckill stock | String (counter) | `seckill:stock:{voucherId}` |
| Seckill user dedup | Set | `seckill:order:{voucherId}` |
| Blog likes | ZSet | `blog:liked:{blogId}` |
| Follow relationships | Set | `follow:{userId}` |
| Feed timeline | ZSet | `feed:{userId}` |
| Shop geo-location | GEO | `shop:geo:{typeId}` |
| User sign-in | BitMap | `sign:{userId}:{yyyyMM}` |
| ID generation | String (INCR) | `icr:{prefix}:{date}` |
| Distributed lock | String (SETNX) | `lock:{name}` |

## Testing Notes

- Default tests are isolated and run without MySQL, Redis, or Kafka.
- Infrastructure tests use `//go:build integration`; load tests use `//go:build load`.
- Data preparation utilities use `//go:build manual`; they are never part of the integration suite.
- No testify/gomock — standard `testing` package only.
- `TestMain` explicitly calls `global.Init` only in tagged integration/load suites.
- Tagged concurrency tests verify V1 DB results and V2/V3 Redis admission; V3 uses a recording fake writer and does not claim end-to-end Kafka throughput.
- Test helpers: `setupSeckillData`, `cleanupSeckillData`, `setUserContext` in `voucher_order_test.go`.
