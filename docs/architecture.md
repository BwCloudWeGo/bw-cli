# 架构说明

## 目标

本脚手架用于快速启动企业级 Go 微服务项目。设计目标是：

- Gin 对外提供 HTTP API。
- gRPC 作为内部服务通信协议。
- Gorm 作为默认 ORM。
- DDD 思路组织业务边界，但包名保持直观。
- 公共能力可独立沉淀到 Git 仓库，通过 `go get` 复用。
- 命令行工具名固定为 `bw-cli`，通过 `go install <repo>/cmd/bw-cli@latest` 安装。
- `bw-cli new` 生成干净框架，不带业务 demo；`bw-cli demo` 生成带 user/note 的演示项目。
- 生成项目会重写项目文档，确保 README、usage、architecture、toolkit、mongodb 与实际目录一致。

## 总体架构

干净项目默认只保留 Gateway 和公共能力：

```text
Client
  -> Gin Gateway
      -> /healthz
      -> /api/v1      # 业务路由命名空间，按需扩展
```

演示项目额外包含 user/note 两个示例服务：

```text
Client
  -> Gin Gateway
      -> UserService gRPC
          -> handler -> service -> model
                      -> repo -> Gorm
      -> NoteService gRPC
          -> handler -> service -> model
                      -> repo -> Gorm
```

## 服务内分层

### model

放业务模型和最稳定的业务规则：

- 实体：例如 `User`、`Note`、`Order`
- 状态：例如 `Draft`、`Published`、`Paid`
- 业务错误：例如 `ErrUserNotFound`、`ErrInvalidOrder`
- 仓储接口：`Repository`

这一层不依赖框架和数据库。

### service

放业务用例，并按文件继续拆清职责：

- 注册用户
- 创建订单
- 发布内容
- 审核资源

```text
internal/<service>/service/command.go  # 业务用例入参命令
internal/<service>/service/dto.go      # 业务用例出参 DTO 和转换
internal/<service>/service/service.go  # 业务流程编排
```

`command.go` 只定义 `CreateCommand`、`UpdateCommand` 等入参，不写业务流程。`dto.go` 只定义返回结构和 `toDTO` 转换，不暴露领域模型或数据库模型。`service.go` 只写用例方法，负责调用领域模型和 `model.Repository`。

这一层只依赖 `model` 中的接口和实体。事务、幂等、权限等业务编排也放在这里。

### repo

放基础设施实现：

- Gorm Model
- Gorm Repository
- 数据库迁移
- Redis、ES、Kafka 等外部依赖适配

这一层实现 `model.Repository` 接口。

### handler

放入站协议适配：

- gRPC server
- HTTP handler
- DTO 转换
- 错误码转换

这一层调用 `service`，不直接操作数据库。

## Gateway 分层

Gateway 不把所有路由放在一个文件里，也不把请求入参写在控制器里：

```text
internal/gateway
  └── router
      ├── router.go       # 创建 Gin engine 和全局中间件
      ├── health.go       # /healthz
      └── v1.go           # /api/v1
```

新增业务后再按需创建：

```text
internal/gateway
  ├── request
  │   └── product_request.go
  ├── handler
  │   └── product_handler.go
  └── router
      └── product_routes.go
```

路由注册顺序固定为：

```text
版本 -> 业务 -> 具体接口
/api -> /v1 -> /products -> /:id
/api -> /v1 -> /orders -> /:id/pay
```

Handler 只做协议适配：绑定 `request` 包中的 DTO、调用 gRPC client、输出统一响应。业务校验和状态变化放在下游服务的 `service/model` 中。

## 公共包

```text
pkg/config       配置加载
pkg/logger       结构化日志和文件轮转
pkg/errors       统一错误码
pkg/middleware   Gin 中间件
pkg/grpcx        gRPC 拦截器
pkg/httpx        HTTP 响应封装
pkg/mysqlx       MySQL/Gorm 封装
pkg/postgresx    PostgreSQL/Gorm 封装
pkg/mongox       MongoDB 官方驱动封装
pkg/redisx       Redis 封装
pkg/esx          Elasticsearch 封装
pkg/kafkax       Kafka 封装
pkg/filex        文件上传校验和对象存储封装
pkg/scaffold     脚手架生成逻辑
```

## 日志设计

每个进程独立日志文件，文件名按服务名和当前日期生成：

```text
logs/gateway-2026-04-28.log
logs/user-service-2026-04-28.log
logs/note-service-2026-04-28.log
```

保留策略：

- `max_age_days: 7`
- `max_size_mb: 128`
- `max_backups: 14`
- `compress: true`

日志维度：

- 网关层：请求路径、路由模板、状态码、耗时、请求大小、响应大小、客户端 IP。
- gRPC 层：RPC 方法、状态码、peer、耗时、request_id。
- 业务层：用例名、聚合 ID、用户 ID、业务错误码。
- 仓储层：repository、operation、rows_affected、latency_ms、error。

## 中间件

当前内置：

- `RequestID`：生成或透传 `X-Request-ID`。
- `RequestLogger`：记录 HTTP 请求日志。
- `CORS`：支持来源、方法、请求头、凭证和 max age 配置。
- `JWTAuth`：解析 Bearer token，并把 claims 写入 Gin context。

JWT token 可以通过 `middleware.GenerateToken` 生成。密钥必须来自 `configs/config.yaml` 或环境变量，默认不提供假密钥。

## 外部组件封装

MySQL、PostgreSQL、MongoDB、Redis、ES、Kafka、文件上传都在 `pkg` 下提供薄封装，目标不是隐藏原生 SDK，而是统一默认配置、连接初始化、连接池、调用入口和可替换的 provider。

关系型数据库统一通过 `pkg/database.Open` 进入，目前支持：

```text
database.driver=sqlite
database.driver=mysql
database.driver=postgres
database.driver=postgresql
database.driver=pg
```

MongoDB 是文档数据库，不走 Gorm 入口。业务服务需要 MongoDB 时，从 `config.MongoDB` 读取配置并调用 `mongox.NewClient` 创建客户端，再在 `repo` 层封装集合、索引和查询逻辑。

文件上传通过 `pkg/filex` 进入，业务层只依赖 `filex.Uploader` 接口。上传前会校验文件大小、扩展名和 MIME 类型，默认最大 100 MB，默认支持 Word、PDF、常见图片、视频和音频格式。当前存储 provider 支持：

```text
file_storage.provider=minio
file_storage.provider=oss
file_storage.provider=qiniu
file_storage.provider=cos
```

对象 key 默认按日期分区生成：

```text
uploads/YYYY/MM/DD/<uuid>.<ext>
```

如果配置 `file_storage.public_base_url`，上传结果会返回可访问 URL；如果没有配置，则返回空 URL，由业务根据私有桶签名下载策略自行处理。

后续推到 Git 仓库后可以通过：

```bash
go get github.com/BwCloudWeGo/bw-cli/pkg/mysqlx
go get github.com/BwCloudWeGo/bw-cli/pkg/postgresx
go get github.com/BwCloudWeGo/bw-cli/pkg/mongox
go get github.com/BwCloudWeGo/bw-cli/pkg/filex
```

方式复用。若企业内多个项目长期共用，建议拆成独立基础库仓库。

## 扩展新服务

推荐直接使用脚手架生成完整调用链：

```bash
bw-cli service <service> --tidy
```

命令会创建 proto、cmd、model、service、repo、handler、gateway request/handler/router 和服务文档。需要手工扩展时按以下步骤：

1. 在 `api/proto/<service>/v1` 添加 proto。
2. 运行 `make proto`。
3. 创建 `internal/<service>/model`。
4. 创建 `internal/<service>/service/command.go`、`dto.go`、`service.go`。
5. 创建 `internal/<service>/repo`。
6. 创建 `internal/<service>/handler`。
7. 创建 `cmd/<service>/main.go`。
8. 在 gateway 增加 gRPC client 和 HTTP route。
