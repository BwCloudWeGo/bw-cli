# 使用说明

这份文档面向脚手架维护者和使用者。当前仓库只保留脚手架内容，不包含内置业务 demo。

## 维护者流程

```bash
make tools
make proto
make test
```

安装本地 CLI：

```bash
go install ./cmd/bw-cli
```

验证：

```bash
bw-cli new -h
```

## 生成干净项目

```bash
bw-cli new my-service \
  --module github.com/acme/my-service \
  --tidy
```

从本地源码生成：

```bash
bw-cli new ../my-service \
  --module github.com/acme/my-service \
  --source . \
  --tidy
```

生成后：

```bash
cd ../my-service
make proto
make test
make run-gateway
```

如果没有 proto 文件，`make proto` 会输出 `No proto files found` 并退出成功。

## 配置覆盖

主配置文件：

```text
configs/config.yaml
```

环境变量规则：

```text
APP_ + 配置路径大写 + 下划线
```

示例：

```bash
export APP_HTTP_PORT=8081
export APP_LOG_LEVEL=debug
export APP_DATABASE_DRIVER=postgres
```

## 新增服务

```bash
bw-cli service comment --port 9103 --tidy
```

按已有关系型表生成：

```bash
bw-cli service comment --table comments --schema configs/services/comment.yaml --tidy
```

表驱动模式会在写文件前连接数据库并校验表结构；校验失败时不会生成半套文件。更多规则见 [表驱动服务生成](/Users/fuyx/kiro/xiaolanshu/docs/table-driven-service.md)。

生成内容：

```text
api/proto/comment/v1/comment.proto
cmd/comment/main.go
internal/comment/model
internal/comment/dto
internal/comment/service
internal/comment/repo
internal/comment/handler
internal/gateway/request/comment_request.go
internal/gateway/handler/comment_handler.go
internal/gateway/router/comment_routes.go
docs/services/comment.md
```

启动服务：

```bash
make run-comment
```

覆盖端口：

```bash
APP_COMMENT_GRPC_PORT=9104 make run-comment
```

gateway 默认连接 `127.0.0.1:<port>`，也可以覆盖目标：

```bash
APP_COMMENT_GRPC_TARGET=127.0.0.1:9104 make run-gateway
```

## 分层约定

```text
handler -> service -> model
repo -> model
```

- `model` 放实体、业务错误和 Repository 接口。
- `dto/command.go` 放业务用例入参。
- `dto/<service>.go` 放业务出参和领域模型转换。
- `service/service.go` 编排业务流程。
- `repo` 实现 Gorm/MongoDB 等基础设施访问。
- `handler` 只做 gRPC 协议转换和错误映射。

公共包用法见 [工具组件](/Users/fuyx/kiro/xiaolanshu/docs/toolkit.md)。
