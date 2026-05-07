# 按数据库表字段生成服务设计

## 目标

升级 `bw-cli service`，让用户在项目已经配置数据库的前提下，可以指定表名并自动生成完整服务结构：

```bash
bw-cli service goods --table goods --tidy
```

命令读取项目根目录的 `configs/config.yaml`，根据 `database.driver` 当前值自动选择 SQLite、MySQL 或 PostgreSQL 连接方式，读取指定表的真实字段，再生成 proto、model、dto、service、repo、handler、gateway 路由和服务文档。用户不传 `--table` 时保持现有默认 `Name/Description` 示例生成逻辑。

## 非目标

- 不从 MongoDB collection 反推字段。MongoDB 没有稳定 schema，首版仍根据关系型表字段同步生成 Mongo Document。
- 不生成复杂业务规则、外键聚合、跨表 join 或事务流程。
- 不覆盖已经存在的服务目录或文件。
- 不把数据库密码、连接串写入生成代码。

## 命令设计

新增参数：

```bash
bw-cli service <service-name> --table <table-name> [--schema <schema>] [--config configs/config.yaml] [--port 9100] [--tidy]
```

参数说明：

| 参数 | 用途 | 默认值 |
| --- | --- | --- |
| `--table` | 指定要读取的数据库表名 | 空，表示使用默认示例字段 |
| `--schema` | PostgreSQL schema 或 MySQL database 辅助过滤 | PostgreSQL 默认 `public`，MySQL 默认当前 database |
| `--config` | 指定配置文件路径 | `configs/config.yaml` |
| `--port` | 生成服务默认 gRPC 端口 | `9100` |
| `--skip-proto` | 跳过 proto 生成 | false |
| `--tidy` | 生成后执行 `go mod tidy` | false |

## 数据源选择

命令只根据配置文件中的 `database.driver` 选择数据源：

| `database.driver` | 读取配置 | 支持方式 |
| --- | --- | --- |
| `sqlite` | `database.dsn` | `PRAGMA table_info(<table>)` |
| `mysql` | `mysql.dsn` | `information_schema.columns` |
| `postgres` / `postgresql` / `pg` | `postgresql.dsn` | `information_schema.columns` |

如果配置不存在、DSN 为空、连接失败或表不存在，命令直接失败并给出明确错误。

## 字段模型

新增内部结构 `TableColumn` 表达表字段元数据：

```go
type TableColumn struct {
    Name          string
    DBType        string
    Nullable      bool
    PrimaryKey    bool
    AutoIncrement bool
    HasDefault    bool
    Comment       string
}
```

生成前会转换成 `ServiceField`：

```go
type ServiceField struct {
    DBName      string
    GoName      string
    GoType      string
    ProtoName   string
    ProtoType   string
    GormTag     string
    BSONTag     string
    JSONName    string
    IsPrimary   bool
    IsCreateIn  bool
    IsUpdateIn  bool
    IsListOut   bool
}
```

## 字段映射规则

命名转换：

- `id` -> `ID`
- `user_id` -> `UserID`
- `goods_name` -> `GoodsName`
- `created_at` -> `CreatedAt`
- `updated_at` -> `UpdatedAt`

Go 类型映射首版规则：

| 数据库类型 | Go 类型 | Proto 类型 |
| --- | --- | --- |
| char/varchar/text/uuid | `string` | `string` |
| tinyint/smallint/int/integer | `int32` | `int32` |
| bigint | `int64` | `int64` |
| decimal/numeric/float/double/real | `float64` | `double` |
| bool/boolean | `bool` | `bool` |
| date/datetime/timestamp/time | `time.Time` | `string` |
| json/jsonb | `string` | `string` |
| blob/bytea/binary | `[]byte` | `bytes` |

空值规则：

- 非 nullable 字段生成普通类型。
- nullable 字段首版仍生成普通类型，避免业务层大量指针类型；后续可增加 `--nullable-pointer`。
- 有默认值或自增的主键不进入 CreateCommand。

输入输出规则：

- 主键字段用于 Get/Update/Delete。
- 自增主键不进入 CreateCommand。
- `created_at` 不进入 CreateCommand 和 UpdateCommand，由数据库或 repo 控制。
- `updated_at` 不进入 CreateCommand，Update 时由 repo 或数据库控制。
- 其他字段进入 CreateCommand、UpdateCommand 和 DTO。

## 生成文件

传入 `--table goods` 后生成完整结构：

```text
api/proto/goods/v1/goods.proto
cmd/goods/main.go
internal/goods/model/goods.go
internal/goods/model/repository.go
internal/goods/dto/command.go
internal/goods/dto/goods.go
internal/goods/service/service.go
internal/goods/service/service_test.go
internal/goods/repo/gorm_repository.go
internal/goods/repo/mongo_repository.go
internal/goods/handler/server.go
internal/gateway/request/goods_request.go
internal/gateway/handler/goods_handler.go
internal/gateway/router/goods_routes.go
docs/services/goods.md
```

`gorm_repository.go` 使用真实字段生成 Gorm model：

```go
type GoodsModel struct {
    ID        int64     `gorm:"column:id;primaryKey"`
    GoodsName string    `gorm:"column:goods_name"`
    Price     int64     `gorm:"column:price"`
    CreatedAt time.Time `gorm:"column:created_at"`
}
```

`mongo_repository.go` 使用同一套字段生成 Document：

```go
type GoodsDocument struct {
    ID        int64     `bson:"_id"`
    GoodsName string    `bson:"goods_name"`
    Price     int64     `bson:"price"`
    CreatedAt time.Time `bson:"created_at"`
}
```

## 分层要求

生成后仍保持当前工程分层：

```text
handler -> service -> model.Repository -> repo -> database/mongox
```

每层职责：

- `model`：领域实体、业务错误、仓储接口。
- `dto/command.go`：Create/Update/List 等业务用例入参。
- `dto/<service>.go`：出参 DTO 和领域模型转换。
- `service/service.go`：只编排业务流程。
- `repo/gorm_repository.go`：真实表字段的 Gorm 持久化。
- `repo/mongo_repository.go`：同字段 Mongo Document 持久化示例。
- `handler`：只做 proto request/response 转换。

## 错误处理

明确失败场景：

- `--table` 指定但配置文件不存在。
- `database.driver` 不支持。
- DSN 为空。
- 数据库连接失败。
- 表不存在或没有字段。
- 表没有数据库主键但存在 `id` 字段时，把 `id` 作为默认主键继续生成，并在服务文档中标注这个假设。
- 表既没有数据库主键也没有 `id` 字段时命令失败，避免生成不可用 CRUD。

## 测试策略

单元测试：

- 解析 `--table`、`--schema`、`--config` 参数。
- SQLite introspection 读取临时库表字段。
- MySQL/PostgreSQL SQL 构造和字段映射通过 fake rows 或纯函数测试覆盖。
- 字段命名、类型映射、Create/Update 输入字段过滤。
- 不传 `--table` 时保持现有默认服务生成。
- 传 `--table` 时生成的 repo/model/dto/proto 包含真实字段。

集成验证：

- 使用临时 SQLite 数据库创建 `goods` 表。
- 执行 `bw-cli service goods --table goods --config <tmp-config> --skip-proto`。
- 在生成项目中执行 `go test ./internal/goods/...`。

## 文档更新

更新以下文档：

- `README.md`
- `docs/usage.md`
- `docs/architecture.md`
- `docs/toolkit.md`
- 生成的 `docs/services/<service>.md`

文档要写清楚：

- 不传 `--table` 是默认示例服务。
- 传 `--table` 会读取当前配置数据库。
- 如何配置 SQLite/MySQL/PostgreSQL。
- 字段映射规则和常见限制。
