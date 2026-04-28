# bw-cli Go Microservice Scaffold

这是一个企业级 Go 微服务脚手架，默认使用 Gin + gRPC + Gorm，按 DDD 思路组织代码，但包名保持通俗易懂。当前包含两个 demo 服务：`user-service` 和 `note-service`。

## 快速体验

```bash
make tools
make proto
make test
```

分别启动三个进程：

```bash
make run-user
make run-note
make run-gateway
```

健康检查：

```bash
curl http://localhost:8080/healthz
```

## 分层命名

每个业务服务统一四层：

```text
internal/<service>
  ├── model    # 实体、值对象、业务错误、仓储接口
  ├── service  # 业务用例编排
  ├── repo     # Gorm、Redis、外部依赖实现
  └── handler  # gRPC/HTTP 入站适配
```

Gateway 额外拆出请求 DTO 和分层路由：

```text
internal/gateway
  ├── request          # HTTP 入参结构体
  ├── handler          # HTTP 控制器，只负责绑定、调用、响应
  └── router
      ├── v1.go        # /api/v1 版本分组
      ├── user_routes.go
      └── note_routes.go
```

依赖方向：

```text
handler -> service -> model
repo -> model
```

`model` 不依赖 Gin、gRPC、Gorm、日志框架，方便单元测试和后续替换基础设施。

## 日志

日志默认使用 Zap + Lumberjack：

- 默认保留 7 天
- 单文件最大 128 MB
- 最多保留 14 个备份
- 历史日志压缩
- 文件名按当前日期和服务名生成，例如 `logs/gateway-2026-04-28.log`

记录维度覆盖：

- HTTP：method、path、route、status、client_ip、user_agent、latency_ms、request_bytes、response_bytes、error_code
- gRPC：full_method、peer、status_code、latency_ms、request_id、trace_id、error_code
- 业务：service、env、request_id、user_id、aggregate_id、use_case
- 仓储：repository、operation、rows_affected、latency_ms、error

## 公共组件封装

这些包设计成后续可以放到 Git 仓库里，通过 `go get` 单独引入：

```text
pkg/mysqlx   # Gorm MySQL 初始化和连接池配置
pkg/postgresx # Gorm PostgreSQL 初始化和连接池配置
pkg/mongox   # MongoDB 官方驱动客户端初始化和 ping
pkg/redisx   # go-redis 客户端初始化和 ping
pkg/esx      # Elasticsearch v8 客户端初始化
pkg/kafkax   # Kafka writer/reader 初始化
pkg/logger   # Zap + Lumberjack 日志
pkg/middleware # CORS、JWT、RequestID、请求日志
pkg/grpcx    # gRPC 拦截器和 metadata 透传
pkg/errors   # 统一业务错误码和 HTTP/gRPC 映射
```

推到 Git 仓库后，其他项目可以这样引用：

```bash
go get github.com/BwCloudWeGo/bw-cli/pkg/mysqlx
go get github.com/BwCloudWeGo/bw-cli/pkg/postgresx
go get github.com/BwCloudWeGo/bw-cli/pkg/mongox
go get github.com/BwCloudWeGo/bw-cli/pkg/redisx
go get github.com/BwCloudWeGo/bw-cli/pkg/esx
go get github.com/BwCloudWeGo/bw-cli/pkg/kafkax
```

如果希望更标准，建议后续把 `pkg/*x` 公共组件拆到独立仓库，例如 `github.com/your-org/go-kit`。

## bw-cli 脚手架命令

本仓库提供 `bw-cli` 命令，用来一键生成新项目。命令行工具推荐通过 `go install` 安装；公共基础包通过 `go get` 引入到其他项目。

### 本地拉取脚手架后安装

```bash
git clone https://github.com/BwCloudWeGo/bw-cli.git
cd bw-cli
go install ./cmd/bw-cli
```

安装完成后，在脚手架仓库根目录直接生成项目：

```bash
bw-cli new ../demo-app --module github.com/acme/demo-app --source . --tidy
```

### 通过远程仓库安装并生成项目

脚手架发布到 Git 仓库后，可以直接安装命令：

```bash
go install github.com/BwCloudWeGo/bw-cli/cmd/bw-cli@latest
```

当前 `go.mod` 的 `module` 已经是 `github.com/BwCloudWeGo/bw-cli`，可以直接远程安装。

然后通过 `--repo` 指定脚手架仓库生成项目：

```bash
bw-cli new my-service \
  --module github.com/acme/my-service \
  --repo https://github.com/BwCloudWeGo/bw-cli.git \
  --tidy
```

公共包引用方式：

```bash
go get github.com/BwCloudWeGo/bw-cli/pkg/logger
go get github.com/BwCloudWeGo/bw-cli/pkg/mysqlx
go get github.com/BwCloudWeGo/bw-cli/pkg/postgresx
go get github.com/BwCloudWeGo/bw-cli/pkg/mongox
```

生成命令会做三件事：

1. 从本地目录或 Git 仓库复制脚手架。
2. 跳过 `.git`、`.idea`、`logs`、`data` 等运行时目录。
3. 将 `go.mod` 和源码里的旧 module 路径替换为新项目 module。

## Demo API

注册用户：

```bash
curl -X POST http://localhost:8080/api/v1/users/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","display_name":"Ada","password":"secret123"}'
```

创建笔记：

```bash
curl -X POST http://localhost:8080/api/v1/notes \
  -H 'Content-Type: application/json' \
  -d '{"author_id":"<user_id>","title":"DDD scaffold","content":"Gin plus gRPC demo"}'
```

发布笔记：

```bash
curl -X POST http://localhost:8080/api/v1/notes/<note_id>/publish
```

## 更多文档

- [架构说明](docs/architecture.md)：解释分层、路由、公共包和扩展方式。
- [使用说明](docs/usage.md)：按步骤说明如何发布 `bw-cli`、安装命令、生成项目、初始化依赖、配置服务、启动验证和扩展业务。
- [MongoDB 使用教程](docs/mongodb.md)：说明配置、连接、建模、仓储封装、索引、分页和常见问题。
