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

项目用三个版本展示秒杀链路从同步事务、Channel 入队原型到 Kafka 异步落库的演进，并实现 Redis 登录态、空值缓存与互斥重建、Lua 原子预扣、数据库最终约束和可取消的优雅停机。重点是能解释实现、边界和取舍，而不是用未经验证的数据包装吞吐能力。

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
go mod download
```

### 3. 准备数据库与中间件

确保以下地址可访问（默认与 `config/config.yaml` 一致）：

- MySQL: `127.0.0.1:3306`
- Redis: `127.0.0.1:6379`
- Kafka: `127.0.0.1:9092`

并提前创建数据库与 Topic：

- MySQL 数据库：`hmdp`，建表脚本`hmdp.sql`已提供。
- Kafka Topic：`voucher-order-topic`

如果已经导入过旧版 SQL，需要补上一人一券的数据库最终约束：

```sql
ALTER TABLE tb_voucher_order
ADD UNIQUE INDEX uk_voucher_order_user_voucher (user_id, voucher_id);
```

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

- Redis 连接失败：检查 `redis.host + redis.port` 是否可达（端口字段写 `6379`，代码会拼接冒号）。
- Kafka 无法消费：确认 `group_id`、`topic` 与 broker 地址一致，且 broker 对外地址可被本机访问。
- MySQL 认证失败：检查账号密码、数据库名与字符集配置。

## 测试

### 默认验证（无需 MySQL、Redis、Kafka）

```bash
go test -race -count=1 ./...
```

默认测试覆盖 Kafka 消息构造与错误传播、消费者重试/提交顺序、Channel 取消语义和锁 Key 规则。CI 执行同一条命令。

### 集成测试

集成测试会连接并修改 `config/config.yaml` 指向的 MySQL 和 Redis，只应在独立测试环境运行：

```bash
go test -p=1 -v -count=1 -timeout 180s -tags=integration \
  ./internal/util/ \
  ./internal/service/seckill/ \
  ./internal/service/order/ \
  ./internal/service/
```

### 接入路径压测

压测不会随 `go test ./...` 自动执行：

```bash
go test -v -count=1 -timeout 600s -tags=load \
  -run '^TestLoadTest' ./internal/service/
```

V2/V3 压测使用内存队列或 fake Kafka writer，避免污染业务 Topic；结果只衡量 Redis 准入与应用内处理，不包含 broker 或异步落库，因此不能冒充端到端订单吞吐。

### 生成测试报告

```bash
./scripts/gen-test-report.sh
```

报告写入 `docs/test-report.md`。脚本不会吞掉测试失败；任一测试失败时会以非零状态退出。

### 测试覆盖范围

| 模块 | 测试文件 | 覆盖内容 |
|------|---------|---------|
| KafkaExecutor | `order/kafka_executor_test.go` | 消息 Key/JSON、投递错误传播 |
| Kafka Consumer | `service/voucher_order_consumer_test.go` | 处理失败不提交、提交失败不重复落库、幂等重放 |
| ChannelExecutor | `order/channel_executor_test.go` | 入队、顺序、队列满时响应 Context |
| RedissonLock | `util/redisson_lock_test.go` | Key 规则；集成标签验证加锁与解锁 |
| PreCheck / SyncExecutor | `-tags=integration` | Redis Lua、补偿、MySQL 事务与并发 |
| 接入路径压测 | `-tags=load` | 固定并发上限下的 QPS 与延迟分位数 |

## 架构亮点与核心技术选型

详见 [docs/architecture-highlights.md](docs/architecture-highlights.md)
