# qc 操作手册

`qc` 是一个多数据库命令行工具，支持 MySQL、PostgreSQL、SQLite。所有输出为 JSON，内置安全守卫机制，专为 AI Agent 及开发者设计。

版本：0.3.0

---

## 安装

```bash
make build      # 编译生成 qc
make test       # 运行测试
make lint       # 代码检查
```

---

## 快速入门

在项目目录创建 `.env` 文件（与 docker-compose 等项目工具兼容）：

```bash
echo 'QC_DSN=test:test@123@tcp(127.0.0.1:3306)/mydb' > .env
echo 'QC_DRIVER=mysql' >> .env
```

之后无需每次输入连接信息：

```bash
qc ping                        # 测试连接
qc exec "CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100))"
qc exec "INSERT INTO users (name) VALUES ('Alice')"
qc query "SELECT * FROM users"
qc query --limit 10 --offset 0 "SELECT * FROM users"
qc stream "SELECT * FROM large_table"
```

也可以直接传 DSN（优先级高于 .env）：

```bash
# MySQL
qc ping "user:pass@tcp(127.0.0.1:3306)/db"

# PostgreSQL
qc ping "postgres://user:pass@127.0.0.1:5432/db"

# SQLite
qc query "/data/mydb.sqlite" "SELECT * FROM users"
```

---

## 命令参考

DSN 优先级：命令行参数 > `QC_DSN` 环境变量 > `.env` 文件。全局选项（`--driver`、`--force`、`--limit`、`--timeout`）需放在命令名称之前。

### ping — 连通性检查

```
qc ping [dsn]
```

成功输出：`{"status":"ok"}`

### exec — 执行写操作

```
qc exec [dsn] <sql>
qc exec [dsn] -f <file.sql>             # 从文件批量执行
qc exec [dsn] --force <sql>             # 跳过危险操作确认
```

| 选项 | 说明 |
|------|------|
| `-f, --file <path>` | 读取 SQL 文件批量执行 |
| `--transaction` | 所有语句包装为单个事务 |
| `--continue-on-error` | 遇错继续执行后续语句 |

DROP 和 TRUNCATE 需要交互确认（输入 `yes`），或使用 `--force` 跳过。UPDATE/DELETE 无 WHERE 直接拒绝。

输出：

```json
{"last_insert_id": 1, "rows_affected": 1, "duration_ms": 15}
```

批量执行输出 JSON 数组：

```json
[
  {"statement": "CREATE TABLE...", "rows_affected": 0, "duration_ms": 120},
  {"statement": "INSERT INTO...", "last_insert_id": 1, "rows_affected": 1, "duration_ms": 15}
]
```

### query — 执行查询

```
qc query [dsn] <sql> [--limit N] [--offset N] [--timeout D]
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `--limit N` | 100 | 返回行数上限 |
| `--offset N` | 0 | 跳过前 N 行 |
| `--timeout D` | 30s | 查询超时 |

SQL 中无 LIMIT 时自动追加，上限 1000。`has_more` 为 true 表示数据可能被截断。

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
qc stream [dsn] <sql> [--limit N] [--timeout D]
```

逐行输出 JSON，适合大数据集：

```json
{"id": 1, "name": "Alice"}
{"id": 2, "name": "Bob"}
```

### shell — 交互式 Shell

```
qc shell [dsn] [--limit N] [--timeout D] [--force]
```

维护一个持久连接的交互式 Shell。每条 SQL 返回一行 JSON，`;` 分隔的语句在同一连接上顺序执行。

```bash
# 交互模式
qc shell <dsn>

# 管道模式（AI Agent）
echo "SELECT * FROM users; INSERT INTO logs VALUES(1);" | qc shell <dsn>

# 跨语句事务
echo "BEGIN; INSERT INTO t VALUES(1); COMMIT;" | qc shell <dsn>
```

| 命令 | 说明 |
|------|------|
| `exit` / `quit` | 退出 Shell |
| Ctrl+D | 退出 Shell |
| `--` 前缀 | 行注释（跳过） |

输出 — 每条语句一行 JSON：

```json
{"statement":"SELECT * FROM users","type":"query","result":{"columns":["id"],"rows":[[1]],"row_count":1,"duration_ms":0}}
{"statement":"INSERT INTO logs VALUES(1)","type":"exec","result":{"last_insert_id":1,"rows_affected":1,"duration_ms":2}}
{"statement":"BAD SQL","type":"exec","error":"语法错误..."}
```

交互模式下 stderr 显示 `qc> ` 提示符。管道模式下 stdout 只输出 JSON，日志仅输出 WARN 及以上级别。

### 全局选项

| 选项 | 说明 |
|------|------|
| `--driver <name>` | 指定数据库驱动（mysql/postgres/sqlite） |
| `--force` | 跳过危险操作确认 |
| `--limit N` | 查询返回行数上限（shell/query） |
| `--offset N` | 查询跳过行数（query） |
| `--timeout D` | 查询超时 |
| `--version` | 打印版本号 |

---

## 支持的数据库

| 数据库 | DSN 格式 |
|--------|----------|
| MySQL | `user:pass@tcp(host:port)/db` |
| PostgreSQL | `postgres://user:pass@host:port/db` |
| SQLite | `/path/to/file.db` 或 `:memory:` |

驱动自动识别，也可显式指定：

```bash
qc --driver postgres ping "postgres://..."
```

---

## 连接生命周期

每次 CLI 调用只建立 1 个连接（MaxOpenConns=1），执行完毕后退出时关闭。Go Library 用户仍使用默认值（MaxOpenConns=25），适合长生命周期应用。

### Shell 与单次命令的选择

| 场景 | 推荐方式 |
|------|----------|
| 单条 SELECT/INSERT/UPDATE | `query` 或 `exec` |
| 多语句事务 | `shell` 或 `exec --file --transaction` |
| 需要保持会话状态（SET、临时表） | `shell` |
| AI Agent 连续多条 SQL | `shell`（管道模式） |
| 从文件批量执行 | `exec --file` |

---

## 安全机制

- **LIMIT 强制**：无 LIMIT 时自动追加 100，上限 1000
- **危险操作确认**：DROP/TRUNCATE 需输入 `yes` 确认，`--force` 跳过
- **无条件修改拦截**：UPDATE/DELETE 无 WHERE 直接拒绝
- **命令隔离**：query/stream 只接受 SELECT，写操作被拒绝
- **超时保护**：默认 30s 超时自动取消
- **Shell 模式**：交互场景下自动允许危险操作

---

## .env 配置文件

兼容标准 `.env` 格式（KEY=VALUE，支持 `#` 注释）：

```bash
QC_DSN=user:pass@tcp(127.0.0.1:3306)/mydb
QC_DRIVER=mysql
```

优先级：命令行参数 > 环境变量 > `.env` 文件

---

## Go 库用法

```go
import (
    _ "github.com/xiaoxl/sql-cli/pkg/db/mysql"
    _ "github.com/xiaoxl/sql-cli/pkg/db/postgres"
    _ "github.com/xiaoxl/sql-cli/pkg/db/sqlite"

    "github.com/xiaoxl/sql-cli/pkg/db"
    "github.com/xiaoxl/sql-cli/pkg/config"
)

sess, _ := db.Open("mysql", "user:pass@tcp(127.0.0.1:3306)/db",
    config.WithDefaultLimit(50),
    config.WithQueryTimeout(10*time.Second),
)
defer sess.Close()

ctx := context.Background()
sess.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Alice")
q, _ := sess.Query(ctx, "SELECT * FROM users")
q, _ = sess.QueryWithOffset(ctx, "SELECT * FROM users", 10, 20)
sr, _ := sess.QueryStream(ctx, "SELECT * FROM t")
for sr.Next() { row := sr.Scan() }
tx, _ := sess.Begin(ctx)
tx.Commit(ctx)
```

---

## 常见错误

| 错误 | 解决 |
|------|------|
| `DSN is required` | 设置 `.env`、`QC_DSN` 环境变量或传 DSN 参数 |
| `dangerous operation requires confirmation` | 输入 `yes` 或加 `--force` |
| `UPDATE/DELETE without WHERE clause` | 添加 WHERE 子句 |
| `only SELECT queries are allowed` | 改用 exec 命令 |
| `LIMIT capped to N` | 减小 limit 或用分页 |
| `unknown database driver` | 检查 `--driver` 或 DSN 格式 |
