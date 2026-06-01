- 我需要你组建一个 `team` 来完成这个工程的落地,后续所有任务都由你们团队完成.
- 你的职责是负责团队的运行 (`team-lead`),并在接到需求时合理的分配每一个人的任务.
- 工程配置初始化后你不能再自动新建团队成员,应该通知已有的专项工作者完成任务而不是创建新的团队成员工作.
- 如果需要新建工作成员应当询问我并且告知我为什么需要新成员以及新成员应当具备什么能力.
- 你需要具备的能力是判断任务的工作对象是谁,合理分配给团队人员,当提出bug时不要自己去修复,应该分析bug出现的行为并精准指定工作人员进而推动团队成员去修复bug触发流程,需要总结工作情况更新工作进度,组织团队成员讨论产品方案设计.

### 人员配置如下:

- `product-manager`: 根据用户产品需求描述做出产品企划和目标规划.
- `developer`: 完成项目后端开发(精通 Go 语言工具链,使用 `go-sql-driver/mysql` 作为数据库驱动,可选使用 `sqlx`、`pkg/errors` 等辅助库),同时具备后端架构师的能力.
- `test-engineer`: 负责测试工程,能够在开发人员完成开发后完成测试工程.
- `code-reviewer`: 负责审查代码质量.
技术方案: Go 语言后端库, 数据库驱动 `github.com/go-sql-driver/mysql`, 辅助库 `github.com/jmoiron/sqlx`、`github.com/pkg/errors`, 日志使用 `log/slog`. 如果未明确指明版本号则全部按 `latest` 处理
创建团队人员时,结合本地用户已存在的agents分配能力

### 工作流程如下: 

`product-manager产出项目企划书 => developer进行需求开发 => test-engineer进行测试 => code-reviewer 审查代码(无误则流程进行下一步,有问题则回退到开发流程) => 提交代码`

### 用户产品需求描述

```text
[核心目标]
实现一个 Go 库（或命令行工具，但主要被 Agent 编程调用），提供以下能力：

  1、连接并管理 MySQL 数据库;
  2、执行 DDL / DML 语句（建表、改表、增删改数据）;
  3、支持 事务手动提交（显式 BEGIN、COMMIT、ROLLBACK）;
  4、执行 只读查询，并将结果返回给 Agent 进行分析;
  5、禁止全量查询：必须强制使用 LIMIT 或其他限制手段，防止一次返回海量数据;
  6、强烈建议支持流式查询，避免内存爆炸，让 Agent 可以逐批获取并分析数据;
  7、所有操作面向 Agent 调用设计，返回结构化、易于解析的结果（JSON 等），并包含必要的元信息（耗时、影响行数、错误等）.

[技术依赖]

  1、数据库驱动：github.com/go-sql-driver/mysql;
  2、可选辅助库：github.com/jmoiron/sqlx（方便处理命名参数和结构体映射）、github.com/pkg/errors（错误包装）;
  3、流式查询建议基于 sql.Rows 的 Next() 天然迭代，或使用游标式分页;
  4、项目结构遵循标准 Go 布局：cmd/, pkg/, internal/, api/ 等.

[架构与扩展性设计要求]
  (架构与扩展性设计要求)
    定义 DB 通用接口，包含连接、关闭、执行、查询、事务等核心方法，允许未来实现 PostgreSQL、SQLite 等驱动。MySQL 作为该接口的第一个实现。
  (连接池与多会话)
    支持同时管理多个数据库连接（按连接名区分），Agent 可通过连接名或 DSN 快速获取会话。
  (配置化设计)
    数据库连接信息、查询限制（最大行数、超时）、流式缓冲大小等全部可配置，并提供合理的默认值.

[详细功能需求]

1. 连接管理
Open(driver, dsn string, options ...Option) (*Session, error)
支持连接池参数：最大连接数、空闲连接数、连接最大存活时间
Close() 优雅关闭
Ping() 健康检查

2. DDL / DML 执行
Exec(sql string, args ...interface{}) (Result, error)
返回结构体包含：LastInsertId、RowsAffected、执行耗时
自动识别并拒绝不带 WHERE 条件的 UPDATE/DELETE（可配置开关）
允许执行 CREATE TABLE、ALTER TABLE、INSERT、UPDATE、DELETE 等

3. 手动事务支持
Begin() (*Tx, error)
Tx.Commit() 和 Tx.Rollback()
事务内仍可执行查询、修改，所有操作在同一个事务句柄下进行
事务必须超时自动回滚，避免长事务锁定

4. 安全查询（强制限制）
Query(sql string, args ...interface{}) (*QueryResult, error)
强制规则：
所有 SELECT 必须包含 LIMIT 子句；如果 SQL 中未显式出现 LIMIT，则自动追加 LIMIT <默认值>（如 100），并返回警告
提供单独的 QueryWithLimit 方法允许 Agent 指定每页大小
最大 LIMIT 值受全局配置限制（如 1000），防止 Agent 指定过大值
支持参数化查询，杜绝 SQL 注入

5. 流式查询（重点）
QueryStream(sql string, args ...interface{}) (<-chan StreamRow, <-chan error, cancel func())
或返回一个迭代器对象：StreamResult 支持 Next(), Scan(), Err(), Close()
Agent 可以逐行或分批次（如每次 50 行）获取数据，避免一次加载所有行到内存
流式过程中允许提前取消（context 取消或调用 Close）
流式查询同样强制 LIMIT 限制，且应支持基于游标或偏移的继续获取（可扩展）

6. 结果数据结构
所有查询结果统一为以下 JSON 兼容形式（或直接返回结构化对象，由调用方序列化）：
{
  "columns": ["col1", "col2"],
  "rows": [["val1", "val2"], ...],
  "row_count": 123,
  "duration_ms": 45,
  "warning": "LIMIT 100 applied automatically",
  "has_more": true  // 流式时表示是否还有数据
}
流式场景下可逐行发送 map[string]interface{} 或对应结构。

[安全与可控性约束]
禁止全量查询：任何 SELECT 无 LIMIT 时必须自动增加 LIMIT，并记录告警；代理也可选择直接拒绝执行（配置项）
危险操作拦截：可通过黑名单或白名单模式，限制 DROP、TRUNCATE 等操作，初期仅允许安全 DDL/DML
超时控制：每个执行方法均接受 context.Context，支持 query/exec 超时
敏感数据保护：日志中可对 SQL 参数进行脱敏，防止密码等泄露
资源限制：限制单个会话的并发查询数、内存使用、最大返回行数

[代码要求]
项目应模块化，清晰分离连接、执行、查询、事务、流式逻辑
提供完整的单元测试（使用内存 MySQL mock 或 docker 测试环境）
README 包含：
项目目标与使用场景
快速开始示例（连接、建表、插入、流式查询、事务）
如何扩展新的数据库实现
配置项说明
所有错误要有明确的类型和上下文，便于 Agent 解析
使用 Go 标准库的 log/slog 记录结构化日志
```
