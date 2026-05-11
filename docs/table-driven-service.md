# 表驱动服务生成

`bw-cli service` 支持从已有关系型数据库表生成服务代码。默认不传 `--table` 时，命令仍生成原来的 `Name` / `Description` 基础 CRUD 模板；传入 `--table` 或 `--schema` 后，命令会进入表驱动模式。

## 基本用法

先在 `configs/config.yaml` 中配置关系型数据库：

```yaml
database:
  driver: sqlite
  dsn: data/xiaolanshu.db
```

支持的驱动：

- `sqlite`：读取 `database.dsn`
- `mysql`：读取 `mysql.dsn`
- `postgres`：读取 `postgresql.dsn`

单表生成：

```bash
bw-cli service order --table orders --port 9110
```

命令会在写文件前完成校验：

- 数据库配置是否可用。
- 主表是否存在。
- 主表是否只有一个主键。
- 表字段是否能映射到 Go/proto 类型。

校验失败时不会写入半套服务文件。

## 多表关系 schema

多表关系放在 YAML 文件中：

```yaml
table: demo_orders
resource: order_report
readonly_fields:
  - id
  - created_at
  - updated_at
relations:
  - name: demo_order_items
    table: demo_order_items
    type: has_many
    local_field: id
    foreign_field: order_id
    methods:
      - ListDemoOrderItemsByOrderID
```

执行：

```bash
bw-cli service order-report \
  --table demo_orders \
  --schema configs/services/order_report.yaml \
  --port 9110
```

规则：

- `table` 是主表，生成完整 CRUD。
- `readonly_fields` 会进入 model/DTO/response，但不会进入 Create/Update 入参。
- `relations[].table` 是关联表，必须存在。
- `local_field` 必须在主表存在。
- `foreign_field` 必须在关联表存在。
- `methods` 可指定生成的关系查询方法名。
- 第一版支持 `has_one`、`has_many`、`belongs_to`，当前示例使用 `has_many`。

## 示例服务：order-report

本仓库已经生成了一个多表关系示例服务：

```text
cmd/order_report
api/proto/order_report/v1/order_report.proto
api/gen/order_report/v1
internal/order_report
configs/services/order_report.yaml
docs/services/order_report.md
```

示例表：

```sql
CREATE TABLE demo_orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  customer_name TEXT NOT NULL,
  status TEXT NOT NULL,
  total_amount DECIMAL(10,2) NOT NULL,
  created_at DATETIME,
  updated_at DATETIME
);

CREATE TABLE demo_order_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id INTEGER NOT NULL,
  sku TEXT NOT NULL,
  product_name TEXT NOT NULL,
  quantity INTEGER NOT NULL,
  unit_price DECIMAL(10,2) NOT NULL,
  created_at DATETIME
);
```

主表 `demo_orders` 生成完整 CRUD：

```text
CreateOrderReport
GetOrderReport
ListOrderReports
UpdateOrderReport
DeleteOrderReport
```

关联表 `demo_order_items` 生成内部 model、DTO、Gorm 查询实现和关系查询 RPC：

```text
ListDemoOrderItemsByOrderID
```

关键文件：

```text
internal/order_report/model/demo_order_item.go
internal/order_report/dto/demo_order_item.go
internal/order_report/repo/query_repository.go
internal/order_report/service/relation_service.go
internal/order_report/handler/server.go
```

关系查询默认实现位于 `internal/order_report/repo/query_repository.go`：

```go
func (r *GormRepository) ListDemoOrderItemsByOrderID(ctx context.Context, id int32) ([]*model.DemoOrderItem, error) {
	var records []DemoOrderItemModel
	if err := r.db.WithContext(ctx).Where("order_id = ?", id).Find(&records).Error; err != nil {
		return nil, err
	}
	// ...
}
```

## 启动示例

生成代码已经包含 `Makefile` target：

```bash
make proto
make run-order_report
```

默认 gRPC 端口是 `9110`，可用环境变量覆盖：

```bash
APP_ORDER_REPORT_GRPC_PORT=9111 make run-order_report
```

gateway 默认会挂载主表 CRUD 路由：

```text
POST   /api/v1/demo_orders
GET    /api/v1/demo_orders
GET    /api/v1/demo_orders/:id
PUT    /api/v1/demo_orders/:id
DELETE /api/v1/demo_orders/:id
```

关系查询当前作为 gRPC RPC 生成；如需 HTTP 暴露，可以在 gateway handler/router 中按业务路径补一个薄适配层。

## 类型映射

常用映射规则：

| 数据库类型 | Go 类型 | proto 类型 |
| --- | --- | --- |
| `int`, `integer`, `smallint` | `int32` | `int32` |
| `bigint` | `int64` | `int64` |
| `varchar`, `text`, `char`, `uuid`, `json` | `string` | `string` |
| `bool`, `boolean`, `tinyint(1)` | `bool` | `bool` |
| `decimal`, `numeric` | `string` | `string` |
| `float`, `double`, `real` | `float64` | `double` |
| `date`, `datetime`, `timestamp`, `time` | `time.Time` | `string` |
| `blob`, `binary`, `bytes` | `[]byte` | `bytes` |

## 限制

- 第一版只支持关系型数据库，不做 MongoDB schema introspection。
- 第一版只支持单字段主键。
- 第一版只支持单字段关联。
- 默认不生成复杂 SQL join，而是按关联字段生成 Gorm 查询方法。
- 表字段默认全部进入生成代码；不应暴露的字段需要在 `exclude_fields` 中显式排除。

