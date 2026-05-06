# Order 服务开发说明

本服务由以下命令生成：

~~~bash
bw-cli service order --port 9100
~~~

## 目录结构

~~~text
api/proto/order/v1/order.proto       # gRPC 协议定义
api/gen/order/v1                          # make proto 生成代码
cmd/order/main.go                         # gRPC 服务启动入口
internal/order/model                      # 领域实体和仓储接口
internal/order/dto/command.go             # 业务用例入参命令
internal/order/dto/order.go               # 业务用例出参 DTO 和转换
internal/order/service/service.go         # 业务流程编排
internal/order/repo                       # Gorm 仓储实现
internal/order/handler                    # gRPC 入站适配器
~~~

## 启动

~~~bash
make proto
make run-order
~~~

默认端口是 `9100`，可以通过环境变量覆盖：

~~~bash
export APP_ORDER_GRPC_PORT=9100
~~~

Windows PowerShell：

~~~powershell
$env:APP_ORDER_GRPC_PORT="9100"; make run-order
~~~

## 基础 CRUD

生成后的服务已经提供 Create/Get/List/Update/Delete 的基础调用链：

~~~text
proto RPC -> handler -> service -> model.Repository -> repo(Gorm) -> database
~~~

用户可以直接把示例字段 `Name`、`Description` 替换成真实业务字段，或者在此基础上新增业务方法。

如果项目包含 Gin gateway，命令也会生成 HTTP 入口：

~~~text
POST   /api/v1/orders
GET    /api/v1/orders
GET    /api/v1/orders/:id
PUT    /api/v1/orders/:id
DELETE /api/v1/orders/:id
~~~

gateway 默认调用 `order-service` 的 `127.0.0.1:9100`，无需改配置。如需覆盖目标地址，设置：

~~~bash
export APP_ORDER_GRPC_TARGET=127.0.0.1:9100
~~~

## 开发顺序

1. 在 `api/proto/order/v1/order.proto` 中定义 RPC、Request、Response。
2. 执行 `make proto` 生成 `api/gen/order/v1`。
3. 在 `internal/order/model` 补充领域实体、业务错误和仓储接口。
4. 在 `internal/order/dto/command.go` 写入参，在 `dto/order.go` 写出参和转换，在 `service/service.go` 编排业务用例。
5. 在 `internal/order/repo` 实现数据库访问。
6. 在 `internal/order/handler` 将 gRPC 请求转成业务命令。
7. 在 `internal/gateway/request`、`handler`、`router` 调整 HTTP 入参、控制器和路由。

## 每一层怎么写，为什么这么写

| 层级 | 写什么 | 为什么 |
| --- | --- | --- |
| `api/proto/order/v1` | RPC、Request、Response、`go_package` | 先稳定外部契约，避免内部模型直接暴露 |
| `api/gen/order/v1` | `make proto` 生成代码 | 保持 proto 与 Go 类型一致，不手写 |
| `cmd/order` | 配置、日志、数据库、gRPC server 组装 | main 只负责依赖装配，不写业务 |
| `internal/order/model` | 领域实体、业务错误、Repository 接口 | 业务核心不依赖 Gin、gRPC、Gorm |
| `internal/order/dto/command.go` | 业务用例入参，例如 `CreateCommand`、`UpdateCommand` | handler 只负责组装命令，入参与流程分开 |
| `internal/order/dto/order.go` | 业务用例出参和领域模型转换 | 对外不暴露领域模型和数据库模型 |
| `internal/order/service/service.go` | 用例编排、事务意图、调用仓储接口 | 表达业务流程，依赖接口而不是数据库实现 |
| `internal/order/repo` | Gorm/MongoDB/Redis 等实现 | 数据库访问集中管理，方便替换和测试 |
| `internal/order/handler` | gRPC request/response 适配 | 协议转换和错误映射，不写数据库逻辑 |
| `internal/gateway/request` | HTTP 入参 DTO | 控制器不堆字段，入参校验更清楚 |
| `internal/gateway/handler` | HTTP 控制器 | 只做绑定、调用 gRPC client 和统一响应 |
| `internal/gateway/router` | HTTP 路由 | 按版本/业务拆分，避免路由堆在一个文件 |

## model 层

`model` 放业务核心，不引入 Gorm、Gin、gRPC SDK。

~~~go
package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOrderNotFound = errors.New("order not found")
	ErrInvalidOrder = errors.New("invalid order")
)

type Order struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewOrder(name string, description string) (*Order, error) {
	now := time.Now().UTC()
	return &Order{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
~~~

仓储接口也放在 `model`：

~~~go
package model

import "context"

type Repository interface {
	Save(ctx context.Context, item *Order) error
	FindByID(ctx context.Context, id string) (*Order, error)
	List(ctx context.Context, offset int, limit int) ([]*Order, int64, error)
	Delete(ctx context.Context, id string) error
}
~~~

这样写的原因是：`service` 只关心业务需要的能力，不关心底层用 MySQL、PostgreSQL、MongoDB 还是测试 fake。

## service 层

`dto` 和 `service` 按职责拆开，避免一个文件同时承担入参、出参和流程编排：

~~~text
internal/order/dto/command.go      # CreateCommand、UpdateCommand、ListCommand
internal/order/dto/order.go        # OrderDTO、ListOrderDTO、FromOrder
internal/order/service/service.go  # Service、NewService、Create/Get/List/Update/Delete
~~~

`service.go` 只编排业务流程，只依赖 `model.Repository`。

~~~go
package service

import (
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/order/model"
)

type Service struct {
	repo model.Repository
	log  *zap.Logger
}

func NewService(repo model.Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, log: log}
}
~~~

新增业务时先在 `dto/command.go` 定义命令对象，例如 `CreateCommand`，再在 `service/service.go` 调用领域模型和仓储接口，最后在 `dto/order.go` 做出参转换。这样 handler 不会堆业务判断，service 也更容易写单元测试。

## repo 层：数据库在哪里操作，如何操作

数据库操作只放在 `internal/order/repo`。启动入口 `cmd/order/main.go` 打开数据库并注入 repo：

~~~go
db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
repo := orderrepo.NewGormRepository(db, log)
svc := orderservice.NewService(repo, log)
~~~

Gorm 仓储示例：

~~~go
type OrderModel struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (OrderModel) TableName() string {
	return "orders"
}

type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&OrderModel{})
}
~~~

数据库操作规则：

- `handler` 不直接操作数据库。
- `service` 不直接使用 `*gorm.DB`。
- `model` 不写 Gorm tag，避免领域模型和数据库实现耦合。
- 查询、分页、事务、锁、索引相关实现都放在 `repo`。
- 需要事务时，在 repo 层内部使用 `db.Transaction(func(tx *gorm.DB) error { ... })`。
- 多数据源时保持接口不变，例如 `GormRepository`、`MongoRepository` 都实现 `model.Repository`。

## handler 层

`handler` 只做 gRPC 协议转换：

1. 从 proto request 取字段。
2. 组装 service command。
3. 调用 service。
4. 把 DTO 转成 proto response。
5. 把业务错误转成统一错误。

不要在 handler 中写 SQL、Gorm 查询、复杂业务判断。

## gateway 层

HTTP 入口由脚手架同步生成：

~~~text
internal/gateway/request/order_request.go
internal/gateway/handler/order_handler.go
internal/gateway/router/order_routes.go
~~~

路由按 `版本/业务/具体接口` 拆分，当前业务挂在 `/api/v1/orders`。gateway handler 默认使用 `APP_ORDER_GRPC_TARGET` 覆盖目标地址，不配置时连接 `127.0.0.1:9100`。
