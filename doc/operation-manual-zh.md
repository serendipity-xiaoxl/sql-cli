# qc 操作手册

`qc` 是一个 MySQL 命令行工具，用于安全、结构化的数据库操作。所有输出均为 JSON 格式，内置安全守卫机制，专为 AI Agent 及开发者设计。

版本：0.1.0

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

编译完成后，将项目根目录下的 `qc` 文件放到 `PATH` 中即可使用。

---

## 快速入门

```bash
# 测试连接
qc ping "test:test@123@tcp(115.29.209.119:3306)/test"

# 建表
qc exec "test:test@123@tcp(115.29.209.119:3306)/test" \
  "CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100), age INT)"

# 插入数据
qc exec "test:test@123@tcp(115.29.209.119:3306)/test" \
  "INSERT INTO users (name, age) VALUES ('张三', 25), ('李四', 30)"

# 查询（自动限制返回行数）
qc query "test:test@123@tcp(115.29.209.119:3306)/test" "SELECT * FROM users"

# 分页查询
qc query "test:test@123@tcp(115.29.209.119:3306)/test" \
  --limit 10 --offset 20 "SELECT * FROM users"

# 流式输出大数据集
qc stream "test:test@123@tcp(115.29.209.119:3306)/test" "SELECT * FROM large_table"
```

---

## 命令参考

### 通用约定

- DSN 格式：`用户名:密码@tcp(主机:端口)/数据库名`
- 所有命令输出均为 JSON
- 可通过环境变量 `QC_DSN` 设置默认 DSN

### ping — 连通性检查

```
qc ping <dsn>
```

| 参数 | 必填 | 说明 |
|------|------|------|
| dsn | 否* | 数据库连接串（若设置了 `QC_DSN` 则可省略） |

成功输出：

```json
{"status":"ok"}
```

### exec — 执行写操作

```
qc exec <dsn> <sql>
```

| 参数 | 必填 | 说明 |
|------|------|------|
| dsn | 否* | 数据库连接串 |
| sql | 是 | 要执行的 SQL 语句 |

支持的 SQL 类型：

- `CREATE TABLE` / `ALTER TABLE`
- `INSERT`
- `UPDATE`（必须有 WHERE 条件）
- `DELETE`（必须有 WHERE 条件）

**注意**：`DROP TABLE` 和 `TRUNCATE TABLE` 默认被拦截，`UPDATE`/`DELETE` 不带 WHERE 也会被拒绝。

成功输出示例：

```json
{"last_insert_id": 1, "rows_affected": 1, "duration_ms": 15}
```

### query — 执行查询

```
qc query <dsn> <sql> [选项]
```

| 参数 / 选项 | 必填 | 默认值 | 说明 |
|-------------|------|--------|------|
| dsn | 否* | — | 数据库连接串 |
| sql | 是 | — | SELECT 语句 |
| `--limit N` | 否 | 100 | 返回行数上限 |
| `--offset N` | 否 | 0 | 跳过前 N 行 |
| `--timeout D` | 否 | 30s | 查询超时（如 `10s`、`1m`） |

**关键行为**：

- 如果 SQL 中未写 `LIMIT`，自动追加 `LIMIT 100`
- `--limit` 不能超过全局上限（1000），超出会被截断
- 当返回行数等于 limit 时，`has_more` 为 `true`，表示可能还有数据
- 非 SELECT 语句会被拒绝执行

成功输出示例：

```json
{
  "columns": ["id", "name", "age"],
  "rows": [[1, "张三", 25], [2, "李四", 30]],
  "row_count": 2,
  "duration_ms": 45,
  "warning": "LIMIT 100 applied automatically",
  "has_more": false
}
```

### stream — 流式查询

```
qc stream <dsn> <sql> [选项]
```

| 参数 / 选项 | 必填 | 默认值 | 说明 |
|-------------|------|--------|------|
| dsn | 否* | — | 数据库连接串 |
| sql | 是 | — | SELECT 语句 |
| `--limit N` | 否 | 100 | 返回行数上限 |
| `--timeout D` | 否 | 30s | 查询超时 |

与 `query` 的区别：`stream` 逐行输出 JSON，每行一个 JSON 对象，适合处理大数据集，避免内存中一次性加载全部数据。

输出示例（每行一条）：

```json
{"row": {"id": 1, "name": "张三"}, "index": 0}
{"row": {"id": 2, "name": "李四"}, "index": 1}
```

### 全局选项

| 选项 | 说明 |
|------|------|
| `--version` | 打印版本号 |

### 环境变量

| 变量 | 说明 |
|------|------|
| `QC_DSN` | 默认数据库连接串，设置后命令中的 dsn 参数可省略 |

---

## 安全机制

`qc` 内置多层安全防护，防止误操作和危险查询。

### LIMIT 强制

所有查询自动限制返回行数，防止全量扫描导致内存溢出或数据库压力。

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| 默认 LIMIT | 100 | SQL 中无 LIMIT 时自动追加 |
| 最大 LIMIT | 1000 | 任何查询的 LIMIT 不能超过此值 |

输出中的 `warning` 字段和 `has_more` 字段可帮助判断数据是否被截断。

### 危险操作拦截

以下操作默认被拦截，执行时直接返回错误：

| 操作 | 处理方式 |
|------|----------|
| `DROP TABLE` | 拦截 |
| `TRUNCATE TABLE` | 拦截 |
| `UPDATE` 不带 `WHERE` | 拦截 |
| `DELETE` 不带 `WHERE` | 拦截 |

这是硬性保护，在工具层面彻底杜绝误删表、误清数据等事故。

### 非查询语句隔离

`query` 和 `stream` 命令只接受 SELECT 语句，任何 INSERT/UPDATE/DELETE/DROP 等写操作都会被拒绝，防止用错命令。

### 查询超时

每个查询都有执行超时限制（默认 30 秒），超时自动取消，避免长时间阻塞。

---

## 输出格式详解

### 写操作结果 (exec)

```json
{
  "last_insert_id": 42,
  "rows_affected": 1,
  "duration_ms": 15
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `last_insert_id` | 整数 | 自增主键 ID（INSERT 时有效，为 0 时不显示） |
| `rows_affected` | 整数 | 影响的行数 |
| `duration_ms` | 整数 | 执行耗时（毫秒） |
| `error` | 字符串 | 仅出错时出现 |

### 查询结果 (query)

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
| `columns` | 字符串数组 | 列名列表 |
| `rows` | 二维数组 | 数据行，每行按 columns 顺序排列 |
| `row_count` | 整数 | 实际返回行数 |
| `duration_ms` | 整数 | 查询耗时（毫秒） |
| `warning` | 字符串 | 仅在自动追加 LIMIT 或限流截断时出现 |
| `has_more` | 布尔 | `true` 表示数据可能被截断，还有更多行 |
| `error` | 字符串 | 仅出错时出现 |

### 流式行 (stream)

每行独立输出一个 JSON 对象：

```json
{"row": {"id": 1, "name": "Alice"}, "index": 0}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `row` | 对象 | 列名到值的映射 |
| `index` | 整数 | 行序号（从 0 开始） |

---

## 作为 Go 库使用

如果你的项目是 Go 语言，可以直接引入 `qc` 的库包编程调用。

### 引入依赖

```bash
go get github.com/xiaoxl/sql-cli
```

### 基本用法

```go
import (
    "context"
    "github.com/xiaoxl/sql-cli/pkg/db"
    "github.com/xiaoxl/sql-cli/pkg/config"
)

func main() {
    sess, _ := db.Open("mysql", "user:pass@tcp(127.0.0.1:3306)/mydb",
        config.WithDefaultLimit(50),
        config.WithQueryTimeout(10 * time.Second),
    )
    defer sess.Close()

    ctx := context.Background()

    // 执行写操作
    res, _ := sess.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Alice")
    // res.LastInsertID, res.RowsAffected, res.DurationMs

    // 查询（自动 LIMIT）
    q, _ := sess.Query(ctx, "SELECT * FROM users")
    // q.Columns, q.Rows, q.RowCount, q.Warning, q.HasMore

    // 分页
    q, _ = sess.QueryWithOffset(ctx, "SELECT * FROM users", 10, 20)

    // 流式
    sr, _ := sess.QueryStream(ctx, "SELECT * FROM large_table")
    for sr.Next() {
        row := sr.Scan() // map[string]interface{}
    }

    // 事务
    tx, _ := sess.Begin(ctx)
    tx.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Bob")
    tx.Commit(ctx)
}
```

### 可配置项

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `WithDefaultLimit` | int | 100 | 自动追加的 LIMIT 值 |
| `WithMaxLimit` | int | 1000 | LIMIT 上限 |
| `WithMaxRows` | int | 1000 | 最大返回行数 |
| `WithQueryTimeout` | duration | 30s | 查询超时 |
| `WithMaxOpenConns` | int | 25 | 连接池最大连接数 |
| `WithMaxIdleConns` | int | 5 | 最大空闲连接 |
| `WithConnMaxLifetime` | duration | 5m | 连接复用时间 |
| `WithStreamBatchSize` | int | 50 | 流缓冲区大小 |
| `WithDangerousOpPolicy` | Policy | Block | 危险操作策略 Block/Warn/Allow |
| `WithRejectNoWhere` | bool | true | 拒绝无 WHERE 的 UPDATE/DELETE |
| `WithLogSanitizeParams` | bool | false | 日志参数脱敏 |
| `WithMaxConcurrentQueries` | int | 0 | 并发上限（0=不限制） |

### 多会话管理

```go
reg := registry.NewRegistry()
reg.Open("prod", "mysql", "user:pass@tcp(prod-db:3306)/db")
reg.Open("dev", "mysql", "user:pass@tcp(dev-db:3306)/db")

prod, _ := reg.Get("prod")
names := reg.List() // ["prod", "dev"]
reg.CloseAll()
```

---

## 测试

```bash
make test            # 全量单元测试
make coverage        # 覆盖率报告
go test -race ./...  # 竞态检测
```

使用 Docker MySQL 进行集成测试：

```bash
docker run -d --name qc-mysql \
  -e MYSQL_ROOT_PASSWORD=testpass \
  -e MYSQL_DATABASE=testdb \
  -p 3307:3306 mysql:8

QC_DSN="root:testpass@tcp(127.0.0.1:3307)/testdb" go test ./...

docker stop qc-mysql && docker rm qc-mysql
```

---

## 常见错误

| 错误信息 | 原因 | 解决 |
|----------|------|------|
| `dangerous operation blocked` | 执行了 DROP/TRUNCATE | 确认操作必要后调整策略 |
| `UPDATE/DELETE without WHERE clause` | 写操作缺少 WHERE 条件 | 添加 WHERE 子句 |
| `only SELECT queries are allowed` | 用 query/stream 执行了非查询语句 | 改用 exec 命令 |
| `LIMIT capped to N` | 请求的 LIMIT 超过上限 | 减小 limit 值或使用分页 |
| `transaction is already committed or rolled back` | 重复提交/回滚事务 | 检查事务代码逻辑 |
