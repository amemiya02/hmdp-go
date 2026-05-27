# HMDP-Go

## 目录

- [项目概览](#项目概览)
- [技术栈](#技术栈)
- [Quick Start](#quick-start)
- [测试](#测试)
- [架构亮点与核心技术选型](docs/architecture-highlights.md#架构亮点与核心技术选型)
- [亮点难点](docs/architecture-highlights.md#亮点难点)

## 项目概览

本项目是基于 Go 语言生态对经典 O2O 社交与营销平台（[原黑马点评](https://www.bilibili.com/video/BV1NV411u7GE/)）的深度重构与架构升级。项目模拟了类似大众点评的核心业务场景，完成了从用户鉴权、商户查询、社交探店、到高并发秒杀抢券的完整业务闭环。

在重构过程中，不仅实现了单体架构向高并发场景的平滑演进，更深度结合了 **Go 语言的高并发特性**（Goroutine + Channel）与多种分布式中间件（Redis、Kafka），系统性地解决了分布式会话、缓存雪崩/击穿/穿透、高并发秒杀超卖、分布式锁以及异步削峰等复杂架构痛点。适合作为展示高并发业务落地能力和 Go 生态工程化实践的标杆项目。

- [原版 Java 架构参考](https://github.com/KNeegcyao/dianping)

## 技术栈

- Go 1.25.6
- Gin
- GORM
- MySQL
- Redis
- Lua
- Kafka

## Quick Start

本节目标：让你在本地最短路径跑起来后端服务。

### 1. 前置依赖

- Go: `>= 1.25.6`
- MySQL: `8.x`
- Redis: `6.x/7.x`
- Kafka: `3.x`（或兼容发行版）
- 黑马点评前端：用原项目的`nginx-1.18.0.zip`即可，解压后直接运行`nginx.exe`。

### 2. 拉取依赖

```bash
go mod tidy
```

### 3. 准备数据库与中间件

确保以下地址可访问（默认与 `config/config.yaml` 一致）：

- MySQL: `127.0.0.1:3306`
- Redis: `127.0.0.1:6379`
- Kafka: `127.0.0.1:9092`

并提前创建数据库与 Topic：

- MySQL 数据库：`hmdp`，建表脚本`hmdp.sql`已提供。
- Kafka Topic：`voucher-order-topic`

### 4. 修改配置

编辑 `config/config.yaml`，重点检查以下字段：

- `mysql.username`
- `mysql.password`
- `mysql.dbname`
- `redis.host` / `redis.port`
- `kafka.brokers`
- `kafka.topic`
- `server.port`

### 5. 启动服务

```bash
go run ./cmd/api/main.go
```

启动成功后默认监听：`http://localhost:8081`

### 6. 快速验证

可先调用一个无需登录的接口确认服务可用：

```bash
curl http://localhost:8081/shop-type/list
```

### 常见问题

- Redis 连接失败：检查 `redis.host + redis.port` 是否可达（端口字段是 `:6379` 这种格式）。
- Kafka 无法消费：确认 `group_id`、`topic` 与 broker 地址一致，且 broker 对外地址可被本机访问。
- MySQL 认证失败：检查账号密码、数据库名与字符集配置。

## 测试

### 运行全部测试

```bash
go test -v -count=1 -timeout 180s ./internal/service/seckill/ ./internal/service/order/ ./internal/service/
```

### 运行测试并输出报告到 Markdown 文件

```bash
go test -v -count=1 -timeout 180s \
  ./internal/service/seckill/ \
  ./internal/service/order/ \
  ./internal/service/ \
  2>&1 | go run ./cmd/testreport/main.go > docs/test-report.md
```

或直接使用管道脚本：

```bash
# 生成测试报告
./scripts/gen-test-report.sh
# 报告输出到 docs/test-report.md
```

### 按模块运行

```bash
# 仅运行 PreCheck 单元测试
go test -v ./internal/service/seckill/

# 仅运行 Executor 单元测试
go test -v ./internal/service/order/

# 仅运行集成与并发测试
go test -v ./internal/service/

# 仅运行并发超卖防护测试
go test -v -run "Concurrent" ./internal/service/
```

### 测试覆盖范围

| 模块 | 测试文件 | 覆盖内容 |
|------|---------|---------|
| PreCheck | `seckill/precheck_test.go` | Lua 脚本三种返回值、Rollback 回滚 |
| SyncExecutor | `order/sync_executor_test.go` | 分布式锁、DB 事务、重复下单、并发 |
| ChannelExecutor | `order/channel_executor_test.go` | Channel 写入、满时阻塞 |
| KafkaExecutor | `order/kafka_executor_test.go` | Kafka 发送、序列化、Context 取消 |
| 集成测试 | `service/voucher_order_test.go` | V1/V2/V3 全链路、登录校验、库存检查 |
| 并发测试 | `service/voucher_order_test.go` | 200 并发超卖防护、同用户重复下单 |

### 测试结果

| 维度 | V1 (Sync) | V2 (Channel) | V3 异步 (Kafka Async) | V3 同步 (Kafka Sync) |
|------|-----------|--------------|----------------------|---------------------|
| QPS 上限 | ~50 | ~45,000 | ~45,000 | ~500 |
| P50 延迟 | 0-13ms | 2-8ms | 2-8ms | 7-18ms |
| P99 延迟 | 5-10s (锁超时) | 4-10ms | 4-10ms | ~1s |
| 适用场景 | 低并发、强一致 | 高并发、单实例 | 高并发、多实例、高可靠 | 需要同步确认 |
| 超卖防护 | 分布式锁 + DB 事务 | Redis Lua 预检 | Redis Lua 预检 | Redis Lua 预检 |

## 架构亮点与核心技术选型

详见 [docs/architecture-highlights.md](docs/architecture-highlights.md)

