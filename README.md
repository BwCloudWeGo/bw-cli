# bw-cli Go 微服务脚手架

`bw-cli` 是一个干净的 Go 微服务脚手架仓库。仓库本身只保留脚手架 CLI、项目生成逻辑、公共工具包、proto 生成工具和一个无业务路由的 gateway 骨架；具体业务服务通过命令生成，不再把示例业务代码提交在脚手架源码里。

## 能力

- `bw-cli new` 生成 Gin + gRPC + Gorm 的干净项目。
- `bw-cli service` 在项目中新增可启动的 CRUD 服务骨架。
- `bw-cli service --table/--schema` 可按已有关系型表生成服务代码。
- 公共包覆盖配置、日志、错误、HTTP 响应、中间件、数据库、MongoDB、Redis、ES、Kafka、文件上传等基础能力。
- `tools/protogen` 用 Go 实现 proto 扫描和生成，避免 Makefile 依赖平台特定 shell 写法。

## 维护脚手架

```bash
make tools
make proto
make test
```

当前仓库没有内置业务 proto，`make proto` 在没有 proto 文件时会输出 `No proto files found` 并正常结束。

运行 CLI：

```bash
make run-cli
```

安装 CLI：

```bash
make install-cli
```

## 生成项目

```bash
bw-cli new my-service \
  --module github.com/acme/my-service \
  --tidy
```

生成后的项目包含：

```text
api/proto
api/gen
cmd/gateway
configs
internal/gateway/router
pkg
tools/protogen
docs
```

启动干净 gateway：

```bash
cd my-service
make run-gateway
```

健康检查：

```bash
curl http://localhost:8080/healthz
```

## 新增业务服务

在生成项目根目录执行：

```bash
bw-cli service order --port 9103 --tidy
```

也可以按已有关系型表生成：

```bash
bw-cli service order --table orders --schema configs/services/order.yaml --tidy
```

命令会生成：

```text
api/proto/order/v1/order.proto
cmd/order/main.go
internal/order/model
internal/order/dto
internal/order/service
internal/order/repo
internal/order/handler
internal/gateway/request/order_request.go
internal/gateway/handler/order_handler.go
internal/gateway/router/order_routes.go
docs/services/order.md
```

生成后的服务默认包含基础 CRUD，默认使用 Gorm 仓储，同时生成 MongoDB 仓储实现供业务切换。

## 目录

```text
cmd/bw-cli                 # 脚手架命令入口
cmd/gateway                # 干净 gateway 骨架
configs                    # 脚手架默认配置
internal/gateway/router    # healthz 和 /api/v1 空命名空间
pkg/scaffold               # 项目和服务生成逻辑
pkg/*                      # 公共工具包
tools/protogen             # 平台中立 proto 生成器
docs                       # 使用、架构和工具说明
```

更多细节见 [使用说明](/Users/fuyx/kiro/xiaolanshu/docs/usage.md)、[架构说明](/Users/fuyx/kiro/xiaolanshu/docs/architecture.md)、[工具组件](/Users/fuyx/kiro/xiaolanshu/docs/toolkit.md) 和 [表驱动服务生成](/Users/fuyx/kiro/xiaolanshu/docs/table-driven-service.md)。
