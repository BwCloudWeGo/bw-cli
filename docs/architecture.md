# 架构说明

`bw-cli` 仓库是脚手架本体，不承载具体业务服务。源码保留 CLI、生成器、公共工具包和干净 gateway 骨架，业务代码由 `bw-cli service` 在目标项目里生成。

## 仓库结构

```text
cmd/bw-cli                 # CLI 入口
cmd/gateway                # 干净 gateway 骨架，用于模板和本地验证
configs/config.yaml        # 默认配置
internal/gateway/router    # healthz 和 /api/v1 空命名空间
pkg/scaffold               # new/service 生成逻辑
pkg/config                 # 配置加载
pkg/logger                 # 日志
pkg/middleware             # CORS/JWT/RequestID/请求日志
pkg/database               # Gorm 入口
pkg/mongox                 # MongoDB 封装
pkg/filex                  # 文件上传封装
tools/protogen             # proto 生成工具
```

## 生成项目

`bw-cli new` 从脚手架源码复制模板，重写 module，移除 CLI 自身和 `pkg/scaffold`，留下业务项目需要的公共能力。

默认生成项目只有 gateway：

```text
Client
  -> Gin Gateway
      -> /healthz
      -> /api/v1
```

## 新增服务

`bw-cli service <name>` 在目标项目中生成完整调用链：

```text
cmd/<service>
api/proto/<service>/v1
internal/<service>/model
internal/<service>/dto
internal/<service>/service
internal/<service>/repo
internal/<service>/handler
internal/gateway/request
internal/gateway/handler
internal/gateway/router
docs/services/<service>.md
```

依赖方向：

```text
gRPC handler -> service -> model.Repository
repo(Gorm/MongoDB) -> model
gateway HTTP handler -> generated gRPC client
```

`model` 不依赖 Gin、gRPC、Gorm 或 MongoDB SDK。基础设施细节集中在 `repo`，协议转换集中在 `handler`。

## Gateway

干净 gateway 只注册：

```text
GET /healthz
/api/v1
```

新增服务时，`bw-cli service` 会追加对应的 gateway request、handler 和 route 文件。

## 公共能力

公共能力位于 `pkg`，可随生成项目一起复制，也可以被其他 Go 项目按需引用。它们保持薄封装：统一配置、默认值、初始化和常用操作，不隐藏底层 SDK 的关键能力。
