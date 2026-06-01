# sql-cli 操作手册

## 概述

sql-cli 是一个 Go 语言库和 CLI 工具，用于安全、结构化的 MySQL 数据库操作。专为人工使用和 AI Agent 调用设计，提供 JSON 友好的结果类型、自动安全守卫和流式查询支持。

### 仓库地址

```
github.com/xiaoxl/sql-cli
```

---

## 构建与安装

### 环境要求

- Go 1.22 或更高版本
- MySQL 8.0+（或兼容的 MySQL 数据库）

### 构建 CLI

```bash
# 使用 make（推荐）
make build
./qc --help

# 或手动构建
go build -o qc ./cmd/cli/
./qc --help
```

### 构建库

```bash
go build ./...
```

### 运行测试

```bash
# 全部测试
make test

# 竞态检测
go test ./... -count=1 -race

# 代码覆盖率
make coverage

# 静态检查
make lint

# 清理构建产物
make clean
```

---

## CLI 命令

CLI（`qc`）封装了库功能，所有输出均为 JSON 格式。MySQL DSN 可通过命令行参数或 `SQL_CLI_DSN` 环境变量提供。

### ping —— 健康检查

```
sql-cli ping <dsn>
```

检查数据库连通性。成功返回 `{"status":"ok"}`。

```bash
sql-cli ping "user:pass@tcp(127.0.0.1:3306)/mydb"
```

### exec —— 执行 DDL/DML

```
sql-cli exec <dsn> <sql>
```

执行 CREATE、ALTER、INSERT、UPDATE、DELETE 语句。返回 JSON，包含 `last_insert_id`、`rows_affected`、`duration_ms`。

```bash
sql-cli exec "root:pass@tcp(127.0.0.1:3306)/test" "CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name TEXT)"
sql-cli exec "root:pass@tcp(127.0.0.1:3306)/test" "INSERT INTO users (name) VALUES ('Alice')"
```

### query —— 执行 SELECT

```
sql-cli query <dsn> <sql> [flags]
```

执行 SELECT 查询，自动强制 LIMIT。返回 JSON，包含 `columns`、`rows`、`row_count`、`duration_ms`、`warning`、`has_more`。

参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--limit N` | 100 | 最大返回行数 |
| `--offset N` | 0 | 跳过的行数（分页） |
| `--timeout D` | （会话默认） | 查询超时，如 `30s` |

```bash
# 基本查询（自动追加 LIMIT 100）
sql-cli query "user:pass@tcp(127.0.0.1:3306)/test" "SELECT * FROM users"

# 分页查询
sql-cli query "user:pass@tcp(127.0.0.1:3306)/test" --limit 10 --offset 20 "SELECT * FROM users"
```

### stream —— 流式查询

```
sql-cli stream <dsn> <sql> [flags]
```

执行 SELECT 查询，逐行输出 JSON。适用于大数据集。

```bash
sql-cli stream "user:pass@tcp(127.0.0.1:3306)/test" "SELECT * FROM large_table"
```

输出格式（每行一个 JSON 对象）：

```json
{"row": {"id": 1, "name": "Alice"}, "index": 0}
{"row": {"id": 2, "name": "Bob"}, "index": 1}
```

### 全局参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--version` | false | 打印版本号并退出 |

### 环境变量

| 变量 | 说明 |
|------|------|
| `QC_DSN` | CLI 命令的默认 DSN |

---

## 库 API

### 打开会话

```go
import (
    "github.com/xiaoxl/sql-cli/pkg/db"
    "github.com/xiaoxl/sql-cli/pkg/config"
)

// 基本用法
sess, err := db.Open("mysql", "user:pass@tcp(127.0.0.1:3306)/mydb")

// 带配置选项
sess, err := db.Open("mysql", dsn,
    config.WithDefaultLimit(50),
    config.WithMaxLimit(500),
    config.WithQueryTimeout(10*time.Second),
    config.WithDangerousOpPolicy(guard.PolicyBlock),
)
defer sess.Close()
```

### 配置选项

`pkg/config` 中的所有配置选项：

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithName(n)` | DSN | 会话标识符 |
| `WithRejectNoWhere(b)` | true | 拒绝无 WHERE 的 UPDATE/DELETE |
| `WithDefaultLimit(n)` | 100 | 无 LIMIT 查询的默认值 |
| `WithMaxLimit(n)` | 1000 | 最大允许的 LIMIT 值 |
| `WithMaxRows(n)` | 1000 | 单次查询最大行数 |
| `WithQueryTimeout(d)` | 30s | 默认查询超时 |
| `WithMaxOpenConns(n)` | 25 | 连接池最大连接数 |
| `WithMaxIdleConns(n)` | 5 | 最大空闲连接数 |
| `WithConnMaxLifetime(d)` | 5m | 连接最大复用时间 |
| `WithStreamBatchSize(n)` | 50 | 流通道缓冲区大小 |
| `WithDangerousOpPolicy(p)` | PolicyBlock | DROP/TRUNCATE 处理策略 |
| `WithLogSanitizeParams(b)` | false | 日志中脱敏 SQL 参数 |
| `WithMaxConcurrentQueries(n)` | 0 | 最大并发查询数（0 = 无限制） |

### 执行语句

```go
ctx := context.Background()

// INSERT
res, err := sess.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Alice")
fmt.Printf("LastInsertID: %d\n", res.LastInsertID)

// UPDATE with WHERE（默认拒绝无 WHERE 的 UPDATE）
res, err := sess.Exec(ctx, "UPDATE users SET name = ? WHERE id = ?", "Bob", 1)
fmt.Printf("RowsAffected: %d\n", res.RowsAffected)

// CREATE TABLE
_, err := sess.Exec(ctx, "CREATE TABLE IF NOT EXISTS items (id INT AUTO_INCREMENT PRIMARY KEY, name TEXT)")

// DROP TABLE（默认被拦截）
_, err := sess.Exec(ctx, "DROP TABLE items")
// 返回 guard.ErrDangerousOp

// DROP TABLE with PolicyWarn（允许执行但记录警告）
cfg.DangerousOpPolicy = guard.PolicyWarn
_, err := sess.Exec(ctx, "DROP TABLE items")
```

### 查询数据

```go
// 基本查询（自动追加 LIMIT 100）
res, err := sess.Query(ctx, "SELECT * FROM users")
fmt.Printf("Columns: %v\n", res.Columns)
fmt.Printf("Rows: %v\n", res.Rows)
fmt.Printf("Warning: %s\n", res.Warning) // "LIMIT 100 applied automatically"
fmt.Printf("HasMore: %v\n", res.HasMore) // 达到限制时为 true

// 指定 LIMIT
res, err := sess.QueryWithLimit(ctx, "SELECT * FROM users", 50)

// 分页查询（LIMIT 10 OFFSET 20）
res, err := sess.QueryWithOffset(ctx, "SELECT * FROM users", 10, 20)

// 参数化查询
res, err := sess.Query(ctx, "SELECT * FROM users WHERE id = ?", 42)

// 非 SELECT 语句被拒绝
_, err := sess.Query(ctx, "DELETE FROM users")
// 返回 ErrNonSelectQuery
```

### 流式查询

```go
sr, err := sess.QueryStream(ctx, "SELECT * FROM large_table")
if err != nil {
    log.Fatal(err)
}
defer sr.Close() // 提前关闭以停止流

for sr.Next() {
    row := sr.Scan()
    fmt.Println(row)
    // row 类型为 map[string]interface{}，列名作为键
}

if err := sr.Err(); err != nil {
    log.Fatal(err)
}
```

### 事务

```go
tx, err := sess.Begin(ctx)
if err != nil {
    log.Fatal(err)
}
defer tx.Rollback(ctx) // 安全 —— 已提交后为 no-op

tx.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Charlie")
tx.Exec(ctx, "UPDATE accounts SET balance = balance - 100 WHERE id = ?", 1)

if err := tx.Commit(ctx); err != nil {
    log.Fatal(err)
}
```

事务特性：
- **超时自动回滚**：事务在 `QueryTimeout` 内未提交则自动回滚。
- **安全守卫**：事务内的 Exec/Query 同样受保护（DROP 拦截、无 WHERE 拒绝、LIMIT 强制）。
- **重复提交/回滚**：返回 `ErrTxDone`。

### 多会话注册表

```go
reg := registry.NewRegistry()

// 打开命名会话
reg.Open("prod", "mysql", dsn1)
reg.Open("staging", "mysql", dsn2)

// 按名称获取
sess, err := reg.Get("prod")

// 列出所有会话
names := reg.List() // ["prod", "staging"]

// 关闭单个
reg.Close("staging")

// 关闭全部
reg.CloseAll()
```

---

## 结果格式

所有结果类型均支持直接 JSON 序列化，便于 Agent 解析。

### ExecResult（DDL/DML）

```json
{
  "last_insert_id": 42,
  "rows_affected": 1,
  "duration_ms": 15
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `last_insert_id` | int64 | 最后插入的自增 ID；为 0 时省略 |
| `rows_affected` | int64 | 受影响的行数 |
| `duration_ms` | int64 | 执行耗时（毫秒） |
| `error` | string | 错误信息；为空时省略 |

### QueryResult（SELECT）

```json
{
  "columns": ["id", "name", "email"],
  "rows": [[1, "Alice", "alice@example.com"], [2, "Bob", "bob@example.com"]],
  "row_count": 2,
  "duration_ms": 45,
  "warning": "LIMIT 100 applied automatically",
  "has_more": false
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `columns` | []string | 列名，顺序与查询一致 |
| `rows` | [][]interface{} | 行数据，每行为按列顺序的值切片 |
| `row_count` | int | 返回的行数 |
| `duration_ms` | int64 | 查询执行耗时（毫秒） |
| `warning` | string | 警告信息（如自动追加 LIMIT）；为空时省略 |
| `has_more` | bool | 行数达到限制时为 true（可能还有更多数据） |
| `error` | string | 错误信息；为空时省略 |

### StreamRow

```json
{"row": {"id": 1, "name": "Alice", "email": "alice@example.com"}, "index": 0}
{"row": {"id": 2, "name": "Bob", "email": "bob@example.com"}, "index": 1}
```

每行为一个 JSON 对象：

| 字段 | 类型 | 说明 |
|------|------|------|
| `row` | map[string]interface{} | 列名到值的映射 |
| `index` | int64 | 顺序行索引（从 0 开始） |
| `error` | string | 行级错误；为空时省略 |

---

## 安全特性

### 1. 自动 LIMIT 强制

所有无显式 LIMIT 子句的 SELECT 查询会自动追加 `DefaultLimit`（100）。限制值上限为 `MaxLimit`（1000）。`has_more` 字段指示返回结果之后是否可能还有更多数据。

```go
// 自动追加：SELECT * FROM users LIMIT 100
res, _ := sess.Query(ctx, "SELECT * FROM users")
// Warning: "LIMIT 100 applied automatically"
// HasMore: 返回恰好 100 行时为 true
```

### 2. 危险操作守卫

`DROP TABLE` 和 `TRUNCATE TABLE` 默认被拦截（`PolicyBlock`）。可配置为警告（`PolicyWarn`）或静默放行（`PolicyAllow`）。

```go
// 被拦截
_, err := sess.Exec(ctx, "DROP TABLE users")
// 返回 guard.ErrDangerousOp

// 警告模式（允许执行但记录警告日志）
cfg.DangerousOpPolicy = guard.PolicyWarn
sess.Exec(ctx, "DROP TABLE users")
```

### 3. 无条件修改保护

`UPDATE` 和 `DELETE` 语句缺少 `WHERE` 子句时默认被拒绝。由 `RejectNoWhere` 控制（默认 `true`）。

```go
// 被拒绝
_, err := sess.Exec(ctx, "DELETE FROM users")
// 返回 ErrUnconditionalModify

// 允许
_, err := sess.Exec(ctx, "DELETE FROM users WHERE id = 1")

// 关闭保护
cfg.RejectNoWhere = false
```

### 4. 非 SELECT 查询拒绝

`Query()` 和 `QueryStream()` 拒绝所有非 SELECT/WITH 语句。

### 5. 查询超时

所有查询都有可配置的超时。会话的 `QueryTimeout`（默认 30s）在 context 无截止时间时自动应用。

### 6. 并发限制

`MaxConcurrentQueries`（默认 0 = 无限制）限制单个会话的并发查询数。

---

## 测试指南

### 运行测试

```bash
# 全部测试
go test ./... -count=1

# 竞态检测
go test ./... -count=1 -race

# 覆盖率
go test ./... -count=1 -coverprofile=coverage.out
go tool cover -func=coverage.out
```

### 测试结构

所有测试使用 `github.com/DATA-DOG/go-sqlmock` 模拟 MySQL，无需真实数据库。

```go
func newMockSession(t *testing.T, cfg *config.Config) (*Session, sqlmock.Sqlmock) {
    db, mock, _ := sqlmock.New(sqlmock.MonitorPingsOption(true))
    sdb := sqlx.NewDb(db, "sqlmock")
    s := NewTestSession("test", "mock://", cfg, sdb)
    return s, mock
}
```

### 测试示例

```go
func TestQuerySelectWithoutLimit(t *testing.T) {
    s, mock := newMockSession(t, config.DefaultConfig())

    mock.ExpectQuery("SELECT \\* FROM users LIMIT 100").
        WillReturnRows(sqlmock.NewRows([]string{"name"}))

    res, err := s.Query(context.Background(), "SELECT * FROM users")
    assert.NoError(t, err)
    assert.Equal(t, "LIMIT 100 applied automatically", res.Warning)
    assert.False(t, res.HasMore)

    mock.ExpectClose()
    s.Close()
    mock.ExpectationsWereMet()
}
```

### Docker MySQL 集成测试

需要真实 MySQL 实例时：

```bash
# 启动 MySQL 8.0 容器
docker run -d \
  --name sql-cli-mysql \
  -e MYSQL_ROOT_PASSWORD=testpass \
  -e MYSQL_DATABASE=testdb \
  -p 3307:3306 \
  mysql:8

# 等待 MySQL 就绪
sleep 10

# 运行集成测试
QC_TEST_DSN="root:testpass@tcp(127.0.0.1:3307)/testdb" go test ./... -count=1

# 用完后停止并删除
docker stop sql-cli-mysql && docker rm sql-cli-mysql
```

### 当前覆盖率

| 包 | 覆盖率 |
|----|--------|
| `internal/sanitize` | 100.0% |
| `internal/sqlnorm` | 98.7% |
| `pkg/config` | 92.6% |
| `pkg/db` | 90.4% |
| `pkg/guard` | 100.0% |
| `pkg/registry` | 100.0% |
| `pkg/result` | 100.0% |

---

## 包结构

```
sql-cli/
  cmd/cli/              CLI 入口
  internal/
    sanitize/           SQL 参数脱敏
    sqlnorm/            SQL 规范化（操作类型、WHERE、LIMIT、OFFSET 检测）
      pagination.go     游标分页
  pkg/
    config/             配置系统（函数式选项）
    db/                 核心库（Session、Exec、Query、Stream、Transaction）
      db.go             Database/Tx 接口
      session.go        Session（Open、Close、Ping、Begin、并发控制）
      executor.go       Exec 实现
      query.go          Query、QueryWithLimit、QueryWithOffset
      stream.go         QueryStream（通道式流查询）
      transaction.go    Transaction 封装（自动回滚、安全守卫）
    guard/              危险操作策略执行
    registry/           多会话注册表
    result/             结构化结果类型（ExecResult、QueryResult、StreamResult）
  doc/
    product-plan.md     功能规格与优先级
    operation-manual.md 英文操作手册
    operation-manual-zh.md 本文件
    test-report.md      测试覆盖率报告
```
