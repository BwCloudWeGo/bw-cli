# OrderReport 服务开发说明

本服务由以下命令生成：

~~~bash
bw-cli service order-report --port 9110
~~~

## 目录结构

~~~text
api/proto/order_report/v1/order_report.proto       # gRPC 协议定义
api/gen/order_report/v1                          # make proto 生成代码
cmd/order_report/main.go                         # gRPC 服务启动入口
internal/order_report/model                      # 领域实体和仓储接口
internal/order_report/dto/command.go             # 业务用例入参命令
internal/order_report/dto/order_report.go          # 业务用例出参 DTO 和转换
internal/order_report/service/service.go         # 业务流程编排
internal/order_report/repo                       # Gorm 和 MongoDB 仓储实现
internal/order_report/handler                    # gRPC 入站适配器
~~~

## 启动

~~~bash
make proto
make run-order_report
~~~

默认端口是 `9110`，可以通过环境变量覆盖：

~~~bash
export APP_ORDER_REPORT_GRPC_PORT=9110
~~~

Windows PowerShell：

~~~powershell
$env:APP_ORDER_REPORT_GRPC_PORT="9110"; make run-order_report
~~~

## 基础 CRUD

生成后的服务已经提供 Create/Get/List/Update/Delete 的基础调用链：

~~~text
proto RPC -> handler -> service -> model.Repository -> repo(Gorm) -> database
~~~

默认启动使用 `repo/gorm_repository.go`，无需改配置即可运行。命令同时生成 `repo/mongo_repository.go`，MongoDB 仓储已通过 `mongox.NewDocumentStore[OrderReportDocument]` 接好基础 CRUD；需要切换 MongoDB 时，只替换 `cmd/order_report/main.go` 中注入的 repository。

用户可以直接把示例字段 `Name`、`Description` 替换成真实业务字段，或者在此基础上新增业务方法。

本服务使用 `--table demo_orders` 生成，model、DTO、proto、Gorm/Mongo 字段来自已有表结构。生成器不会默认隐藏敏感字段；如果字段不应该暴露，请在生成前通过 schema 的 `exclude_fields` 排除，或生成后手动调整 proto/DTO。

表驱动模式不会对已有表执行 `AutoMigrate`。多表关系会生成内部 relation model/dto 和 `repo/query_repository.go`、`service/relation_service.go`，默认按关联字段查询，不自动生成复杂 SQL join。


如果项目包含 Gin gateway，命令也会生成 HTTP 入口：

~~~text
POST   /api/v1/demo_orders
GET    /api/v1/demo_orders
GET    /api/v1/demo_orders/:id
PUT    /api/v1/demo_orders/:id
DELETE /api/v1/demo_orders/:id
~~~

gateway 默认调用 `order_report-service` 的 `127.0.0.1:9110`，无需改配置。如需覆盖目标地址，设置：

~~~bash
export APP_ORDER_REPORT_GRPC_TARGET=127.0.0.1:9110
~~~

## 开发顺序

1. 在 `api/proto/order_report/v1/order_report.proto` 中定义 RPC、Request、Response。
2. 执行 `make proto` 生成 `api/gen/order_report/v1`。
3. 在 `internal/order_report/model` 补充领域实体、业务错误和仓储接口。
4. 在 `internal/order_report/dto/command.go` 写入参，在 `dto/order_report.go` 写出参和转换，在 `service/service.go` 编排业务用例。
5. 在 `internal/order_report/repo` 实现数据库访问。
6. 在 `internal/order_report/handler` 将 gRPC 请求转成业务命令。
7. 在 `internal/gateway/request`、`handler`、`router` 调整 HTTP 入参、控制器和路由。

## 每一层怎么写，为什么这么写

| 层级 | 写什么 | 为什么 |
| --- | --- | --- |
| `api/proto/order_report/v1` | RPC、Request、Response、`go_package` | 先稳定外部契约，避免内部模型直接暴露 |
| `api/gen/order_report/v1` | `make proto` 生成代码 | 保持 proto 与 Go 类型一致，不手写 |
| `cmd/order_report` | 配置、日志、数据库、gRPC server 组装 | main 只负责依赖装配，不写业务 |
| `internal/order_report/model` | 领域实体、业务错误、Repository 接口 | 业务核心不依赖 Gin、gRPC、Gorm |
| `internal/order_report/dto/command.go` | 业务用例入参，例如 `CreateCommand`、`UpdateCommand` | handler 只负责组装命令，入参与流程分开 |
| `internal/order_report/dto/order_report.go` | 业务用例出参和领域模型转换 | 对外不暴露领域模型和数据库模型 |
| `internal/order_report/service/service.go` | 用例编排、事务意图、调用仓储接口 | 表达业务流程，依赖接口而不是数据库实现 |
| `internal/order_report/repo` | Gorm/MongoDB/Redis 等实现 | 数据库访问集中管理，方便替换和测试 |
| `internal/order_report/handler` | gRPC request/response 适配 | 协议转换和错误映射，不写数据库逻辑 |
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
	ErrOrderReportNotFound = errors.New("order_report not found")
	ErrInvalidOrderReport = errors.New("invalid order_report")
)

type OrderReport struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewOrderReport(name string, description string) (*OrderReport, error) {
	now := time.Now().UTC()
	return &OrderReport{
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
	Save(ctx context.Context, item *OrderReport) error
	FindByID(ctx context.Context, id string) (*OrderReport, error)
	List(ctx context.Context, offset int, limit int) ([]*OrderReport, int64, error)
	Delete(ctx context.Context, id string) error
}
~~~

这样写的原因是：`service` 只关心业务需要的能力，不关心底层用 MySQL、PostgreSQL、MongoDB 还是测试 fake。

## service 层

`dto` 和 `service` 按职责拆开，避免一个文件同时承担入参、出参和流程编排：

~~~text
internal/order_report/dto/command.go      # CreateCommand、UpdateCommand、ListCommand
internal/order_report/dto/order_report.go   # OrderReportDTO、ListOrderReportDTO、FromOrderReport
internal/order_report/service/service.go  # Service、NewService、Create/Get/List/Update/Delete
~~~

`service.go` 只编排业务流程，只依赖 `model.Repository`。

~~~go
package service

import (
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
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

新增业务时先在 `dto/command.go` 定义命令对象，例如 `CreateCommand`，再在 `service/service.go` 调用领域模型和仓储接口，最后在 `dto/order_report.go` 做出参转换。这样 handler 不会堆业务判断，service 也更容易写单元测试。

## repo 层：数据库在哪里操作，如何操作

数据库操作只放在 `internal/order_report/repo`。脚手架会同时生成两个仓储实现：

~~~text
internal/order_report/repo/gorm_repository.go   # 默认启用，适合 MySQL/PostgreSQL/SQLite
internal/order_report/repo/mongo_repository.go  # 已封装 DocumentStore，适合文档存储
~~~

启动入口 `cmd/order_report/main.go` 默认打开 Gorm 数据库并注入 repo：

~~~go
db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
repo := orderReportrepo.NewGormRepository(db, log)
svc := orderReportservice.NewService(repo, log)
~~~

Gorm 仓储示例：

~~~go
type OrderReportModel struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (OrderReportModel) TableName() string {
	return "demo_orders"
}

type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&OrderReportModel{})
}
~~~

MongoDB 仓储也已经生成，核心是文档结构体自己声明 collection 名称，然后直接创建公共 `DocumentStore`：

~~~go
type OrderReportDocument struct {
	ID          string    `bson:"_id"`
	Name        string    `bson:"name"`
	Description string    `bson:"description"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

func (OrderReportDocument) MongoCollectionName() string {
	return "demo_orders"
}

type MongoRepository struct {
	documents *mongox.DocumentStore[OrderReportDocument]
}

func NewMongoRepository(db *mongo.Database, log *zap.Logger) *MongoRepository {
	return &MongoRepository{
		documents: mongox.NewDocumentStore[OrderReportDocument](db, log),
	}
}
~~~

切换到 MongoDB 时，在 `cmd/order_report/main.go` 中用配置文件创建 Mongo client 和 database，然后把 repo 注入改成：

~~~go
mongoClient, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
if err != nil {
	log.Fatal("create mongodb client failed", zap.Error(err))
}
defer mongoClient.Disconnect(context.Background())

mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)
repo := orderReportrepo.NewMongoRepository(mongoDB, log)
svc := orderReportservice.NewService(repo, log)
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
internal/gateway/request/order_report_request.go
internal/gateway/handler/order_report_handler.go
internal/gateway/router/order_report_routes.go
~~~

路由按 `版本/业务/具体接口` 拆分，当前业务挂在 `/api/v1/demo_orders`。gateway handler 默认使用 `APP_ORDER_REPORT_GRPC_TARGET` 覆盖目标地址，不配置时连接 `127.0.0.1:9110`。
