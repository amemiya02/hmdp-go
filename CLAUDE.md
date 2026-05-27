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
go mod tidy

# Run all seckill/order tests (requires live MySQL, Redis, Kafka)
go test -v -count=1 -timeout 180s ./internal/service/seckill/ ./internal/service/order/ ./internal/service/

# Run tests by module
go test -v ./internal/service/seckill/        # PreCheck unit tests
go test -v ./internal/service/order/           # Executor unit tests
go test -v ./internal/service/                 # Integration + concurrency tests
go test -v -run "Concurrent" ./internal/service/  # Oversell protection tests only

# Generate test report
./scripts/gen-test-report.sh
```

## Architecture

**Layered design:** Handler → Service → Repository, with manual constructor injection (no DI framework).

- `cmd/api/main.go` — Entry point. Starts Kafka consumer goroutine, sets up Gin routes, graceful shutdown.
- `internal/handler/` — HTTP handlers. Parse requests, bind params, return JSON via `dto.Result`.
- `internal/service/` — Business logic. Orchestrates repos, Redis, caching.
- `internal/repository/` — Pure GORM queries. Stateless structs.
- `internal/model/entity/` — GORM models mapping to `tb_*` tables.
- `internal/model/dto/` — DTOs: `Result` (API response wrapper), `UserDTO`, `LoginForm`, `ScrollResult`.
- `internal/global/` — Singleton clients initialized via `init()`: MySQL, Redis, Kafka, slog.
- `internal/constant/` — Redis key prefixes/TTLs, system constants, regex patterns.
- `internal/util/` — Cache strategies, distributed locks, ID generator, user context helper.
- `internal/middleware/` — Two-layer auth: global `RefreshTokenInterceptor` + route-level `LoginInterceptor`.
- `internal/service/seckill/` — Redis+Lua atomic pre-check + rollback for flash sales.
- `internal/service/order/` — Strategy pattern: `Executor` interface with Sync (V1), Channel (V2), Kafka (V3) implementations. V3 (Kafka) is the production path.

## Configuration

Viper loads `config/config.yaml` at package init time. Config path resolved via `runtime.Caller(0)` so tests work too. Sub-structs: `ServerConfig`, `MySQLConfig`, `RedisConfig`, `KafkaConfig`.

## Seckill Flow (Key Feature)

Three evolutionary versions of the same seckill operation, all sharing the same Redis Lua pre-check:

1. **V1 (Sync):** Lua pre-check → distributed lock (Redisson-style with watchdog) → DB transaction
2. **V2 (Channel):** Lua pre-check → buffered Go channel → consumer goroutine
3. **V3 (Kafka, production):** Lua pre-check → Kafka topic → background consumer → DB transaction

The `seckill.lua` script atomically checks stock > 0, checks user not in Set, deducts stock, adds user to Set. Returns 0/1/2 for success/stock-empty/duplicate.

## Cache Strategies (`internal/util/cache.go`)

- `QueryWithPassThrough` — Cache-aside + null-value caching (anti-penetration)
- `QueryWithMutex` — Distributed mutex lock (anti-breakdown, time-for-space)
- `QueryWithLogicalExpire` — Logical expiration + async goroutine rebuild (anti-breakdown, space-for-time)

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

- Tests are **integration tests** requiring live MySQL, Redis, and Kafka.
- No testify/gomock — standard `testing` package only.
- Config is loaded via blank import: `import _ "github.com/amemiya02/hmdp-go/config"`.
- Concurrency tests use 200 goroutines with `sync.WaitGroup` + `sync.Mutex` to verify oversell prevention.
- Test helpers: `setupSeckillData`, `cleanupSeckillData`, `setUserContext` in `voucher_order_test.go`.
