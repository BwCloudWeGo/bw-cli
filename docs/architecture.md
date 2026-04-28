# 架构说明

## 目标

本脚手架用于快速启动企业级 Go 微服务项目。设计目标是：

- Gin 对外提供 HTTP API。
- gRPC 作为内部服务通信协议。
- Gorm 作为默认 ORM。
- DDD 思路组织业务边界，但包名保持直观。
- 公共能力可独立沉淀到 Git 仓库，通过 `go get` 复用。
- 命令行工具名固定为 `bw-cli`，通过 `go install <repo>/cmd/bw-cli@latest` 安装。

## 总体架构

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

- 实体：`User`、`Note`
- 状态：`NoteStatusDraft`、`NoteStatusPublished`
- 业务错误：`ErrUserNotFound`、`ErrInvalidNote`
- 仓储接口：`Repository`

这一层不依赖框架和数据库。

### service

放业务用例：

- 注册用户
- 用户登录
- 创建笔记
- 发布笔记

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
  ├── request
  │   ├── user_request.go
  │   └── note_request.go
  ├── handler
  │   ├── user_handler.go
  │   └── note_handler.go
  └── router
      ├── router.go       # 创建 Gin engine 和全局中间件
      ├── health.go       # /healthz
      ├── v1.go           # /api/v1
      ├── user_routes.go  # /api/v1/users
      └── note_routes.go  # /api/v1/notes
```

路由注册顺序固定为：

```text
版本 -> 业务 -> 具体接口
/api -> /v1 -> /users -> /register
/api -> /v1 -> /notes -> /:id/publish
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
pkg/redisx       Redis 封装
pkg/esx          Elasticsearch 封装
pkg/kafkax       Kafka 封装
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

MySQL、Redis、ES、Kafka 都在 `pkg` 下提供薄封装，目标不是隐藏原生 SDK，而是统一默认配置、连接初始化、连接池和调用入口。

后续推到 Git 仓库后可以通过：

```bash
go get github.com/BwCloudWeGo/bw-cli/pkg/mysqlx
```

方式复用。若企业内多个项目长期共用，建议拆成独立基础库仓库。

## 扩展新服务

新增服务时按以下步骤：

1. 在 `api/proto/<service>/v1` 添加 proto。
2. 运行 `make proto`。
3. 创建 `internal/<service>/model`。
4. 创建 `internal/<service>/service`。
5. 创建 `internal/<service>/repo`。
6. 创建 `internal/<service>/handler`。
7. 创建 `cmd/<service>/main.go`。
8. 在 gateway 增加 gRPC client 和 HTTP route。
