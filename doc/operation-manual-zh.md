# qc 操作手册

`qc` 是一个多数据库命令行工具，支持 MySQL、PostgreSQL、SQLite。所有输出为 JSON，内置安全守卫机制，专为 AI Agent 及开发者设计。

版本：0.2.0

---

## 安装

### 环境要求

- Go 1.22+

### 编译

```bash
make build      # 编译生成 qc 二进制文件
make test       # 运行测试
make lint       # 代码检查
```

编译完成后将 `qc` 放入 `PATH` 即可使用。

---

## 快速入门

```bash
# MySQL
qc ping "test:test@123@tcp(127.0.0.1:3306)/mydb"
qc exec "test:test@123@tcp(127.0.0.1:3306)/mydb" "CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100))"
qc query "test:test@123@tcp(127.0.0.1:3306)/mydb" "SELECT * FROM users"

# PostgreSQL（DSN 自动识别）
qc ping "postgres://user:pass@127.0.0.1:5432/mydb"
qc query "postgres://user:pass@127.0.0.1:5432/mydb" "SELECT * FROM users"

# SQLite
qc query "/data/mydb.sqlite" "SELECT * FROM users"
qc query ":memory:" "SELECT 1"
```

---

## 支持的数据库

| 数据库 | DSN 格式 | 驱动 |
|--------|----------|------|
| MySQL | `user:pass@tcp(host:port)/db` | `mysql` |
| PostgreSQL | `postgres://user:pass@host:port/db` | `postgres` / `pgx` |
| SQLite | `/path/to/file.db` 或 `:memory:` | `sqlite` / `sqlite3` |

DSN 驱动类型自动识别，也可通过 `--driver` 显式指定：

```bash
qc --driver postgres "postgres://..." query "SELECT 1"
```

---

## 命令参考

### 通用约定

- 所有命令输出均为 JSON
- DSN 可省略（设置 `QC_DSN` 环境变量后）
- 可通过 `--driver` 显式指定数据库类型

### ping — 连通性检查

```
qc ping <dsn>
```

成功输出：`{"status":"ok"}`

### exec — 执行写操作

```
qc exec <dsn> <sql>
qc exec <dsn> -f <file.sql>        # 从文件读取 SQL
qc exec <dsn> -f <file.sql> --transaction  # 事务包裹
qc exec <dsn> -f <file.sql> --continue-on-error  # 遇错继续
```

| 参数 / 选项 | 必填 | 说明 |
|-------------|------|------|
| dsn | 否* | 数据库连接串 |
| sql | 否** | SQL 语句（与 -f 二选一） |
| `-f, --file` | 否 | 读取 SQL 文件批量执行 |
| `--transaction` | 否 | 所有语句包装为单个事务 |
| `--continue-on-error` | 否 | 遇错继续执行后续语句 |
| `--force` | 否 | 跳过危险操作确认提示 |

支持的 SQL：CREATE TABLE、ALTER TABLE、INSERT、UPDATE、DELETE。DROP/TRUNCATE 需交互确认（或使用 `--force`）。

成功输出：

```json
{"last_insert_id": 1, "rows_affected": 1, "duration_ms": 15}
```

### query — 执行查询

```
qc query <dsn> <sql> [选项]
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `--limit N` | 100 | 返回行数上限 |
| `--offset N` | 0 | 跳过前 N 行 |
| `--timeout D` | 30s | 查询超时 |

SQL 中无 LIMIT 时自动追加 `LIMIT 100`，上限 1000。返回行数等于 limit 时 `has_more` 为 true。

输出：

```json
{
  "columns": ["id", "name"],
  "rows": [[1, "Alice"], [2, "Bob"]],
  "row_count": 2,
  "duration_ms": 45,
  "warning": "LIMIT 100 applied automatically",
  "has_more": false
}
```

### stream — 流式查询

```
qc stream <dsn> <sql> [--limit N] [--timeout D]
```

逐行输出 JSON，适合大数据集。每行一个 JSON 对象：

```json
{"row": {"id": 1, "name": "Alice"}, "index": 0}
```

### 环境变量

| 变量 | 说明 |
|------|------|
| `QC_DSN` | 默认数据库连接串 |

### 全局选项

| 选项 | 说明 |
|------|------|
| `--driver <name>` | 指定数据库驱动 |
| `--force` | 跳过危险操作确认 |
| `--version` | 打印版本号 |

---

## 安全机制

### LIMIT 强制

所有查询自动限制返回行数。无 LIMIT 时自动追加 100，上限 1000。`has_more` 标识数据是否可能被截断。

### 危险操作确认

DROP TABLE 和 TRUNCATE TABLE 默认要求交互确认（输入 `yes`），`--force` 可跳过。UPDATE/DELETE 无 WHERE 则直接拒绝。

### 命令隔离

query 和 stream 只接受 SELECT，写操作会被拒绝。

### 查询超时

默认 30 秒超时，超时自动取消。

---

## 作为 Go 库使用

```go
import (
    _ "github.com/xiaoxl/sql-cli/pkg/db/mysql"     // MySQL
    _ "github.com/xiaoxl/sql-cli/pkg/db/postgres"   // PostgreSQL
    _ "github.com/xiaoxl/sql-cli/pkg/db/sqlite"     // SQLite

    "github.com/xiaoxl/sql-cli/pkg/db"
    "github.com/xiaoxl/sql-cli/pkg/config"
    "github.com/xiaoxl/sql-cli/pkg/registry"
)

// MySQL
sess, _ := db.Open("mysql", "user:pass@tcp(127.0.0.1:3306)/db",
    config.WithDefaultLimit(50),
    config.WithQueryTimeout(10 * time.Second),
)

// PostgreSQL
sess, _ := db.Open("postgres", "postgres://user:pass@127.0.0.1:5432/db")

// SQLite
sess, _ := db.Open("sqlite", "/data/mydb.sqlite")

defer sess.Close()
ctx := context.Background()

// 写操作
res, _ := sess.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Alice")

// 查询
q, _ := sess.Query(ctx, "SELECT * FROM users")
// q.Columns, q.Rows, q.RowCount, q.Warning, q.HasMore

// 分页
q, _ = sess.QueryWithOffset(ctx, "SELECT * FROM users", 10, 20)

// 流式
sr, _ := sess.QueryStream(ctx, "SELECT * FROM large_table")
for sr.Next() { row := sr.Scan() }

// 事务
tx, _ := sess.Begin(ctx)
tx.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Bob")
tx.Commit(ctx)

// 多会话管理
reg := registry.NewRegistry()
reg.Open("prod", "mysql", "user:pass@tcp(prod:3306)/db")
reg.Open("dev", "mysql", "user:pass@tcp(dev:3306)/db")
prod, _ := reg.Get("prod")
reg.CloseAll()
```

### 可配置项

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithDefaultLimit(n)` | 100 | 自动 LIMIT |
| `WithMaxLimit(n)` | 1000 | LIMIT 上限 |
| `WithQueryTimeout(d)` | 30s | 查询超时 |
| `WithMaxOpenConns(n)` | 25 | 最大连接数 |
| `WithMaxIdleConns(n)` | 5 | 最大空闲连接 |
| `WithDangerousOpPolicy(p)` | PolicyPrompt | 危险操作策略 |
| `WithRejectNoWhere(b)` | true | 拒绝无 WHERE 修改 |

---

## 常见错误

| 错误 | 原因 | 解决 |
|------|------|------|
| `dangerous operation requires confirmation` | DROP/TRUNCATE 需确认 | 输入 `yes` 或加 `--force` |
| `UPDATE/DELETE without WHERE clause` | 缺少 WHERE | 添加 WHERE 子句 |
| `only SELECT queries are allowed` | 用错了命令 | 改用 exec |
| `LIMIT capped to N` | LIMIT 超过上限 | 减小值或用分页 |
| `transaction is already committed or rolled back` | 重复提交/回滚 | 检查事务逻辑 |
| `unknown database driver` | 驱动未注册 | 检查 import 是否正确 |
| `unrecognized DSN format` | DSN 格式无法识别 | 用 `--driver` 显式指定 |
