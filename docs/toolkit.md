# 工具组件总览

当前仓库保留的是脚手架和公共工具包。业务服务通过 `bw-cli service` 生成后再调用这些包。

## 包列表

| 包 | 能力 |
| --- | --- |
| `pkg/config` | YAML 配置加载、`APP_` 环境变量覆盖、默认值 |
| `pkg/logger` | Zap 日志、按日期文件名、Lumberjack 轮转 |
| `pkg/errors` | 统一业务错误码，HTTP/gRPC 状态映射 |
| `pkg/httpx` | Gin HTTP 统一响应 |
| `pkg/middleware` | CORS、JWT、RequestID、HTTP 请求日志 |
| `pkg/grpcx` | gRPC request_id 透传和日志拦截器 |
| `pkg/database` | SQLite/MySQL/PostgreSQL Gorm 统一入口 |
| `pkg/mysqlx` | MySQL Gorm 初始化 |
| `pkg/postgresx` | PostgreSQL Gorm 初始化 |
| `pkg/mongox` | MongoDB client、Ping、Database、DocumentStore CRUD |
| `pkg/redisx` | Redis client 初始化 |
| `pkg/esx` | Elasticsearch client 初始化 |
| `pkg/kafkax` | Kafka reader/writer 初始化 |
| `pkg/filex` | MinIO/OSS/Qiniu/COS 文件上传 |
| `pkg/validator` | 轻量校验函数 |
| `pkg/scaffold` | CLI 内部的项目和服务生成逻辑 |

## 初始化顺序

```text
config.InitGlobal
  -> logger.New
  -> database.Open / mongox.NewClient / redisx.NewClient
  -> repo/service/handler
  -> Gin 或 gRPC server
```

## 配置

```go
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()
```

环境变量覆盖示例：

```bash
export APP_HTTP_PORT=8081
export APP_DATABASE_DRIVER=postgres
export APP_MIDDLEWARE_JWT_SECRET=replace-with-real-secret
```

## JWT

JWT 使用实例 API：

```go
jwtMiddleware := middleware.NewJWT(cfg.Middleware.JWT)
token, err := jwtMiddleware.GenerateToken(middleware.JWTClaims{
    UserID: "user-id",
    Role:   "admin",
})
```

Gin 路由中使用：

```go
auth := r.Group("/api/v1")
auth.Use(jwtMiddleware.Auth())
```

读取 claims：

```go
claims := middleware.ClaimsFromContext(c)
```

## 数据库

```go
db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
if err != nil {
    log.Fatal("open database failed", zap.Error(err))
}
```

支持 `sqlite`、`mysql`、`postgres`、`postgresql` 和 `pg`。

## MongoDB

```go
type Document struct {
    ID string `bson:"_id"`
}

func (Document) MongoCollectionName() string {
    return "documents"
}

client, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
if err != nil {
    panic(err)
}
defer client.Disconnect(context.Background())

db := mongox.Database(client, cfg.MongoDB.Database)
documents := mongox.NewDocumentStore[Document](db, log)
_, err = documents.UpsertByID(context.Background(), "doc-1", &Document{ID: "doc-1"})
```

## 文件上传

```go
uploader, err := filex.NewUploader(cfg.FileStorage)
if err != nil {
    panic(err)
}
```

支持 provider：`minio`、`oss`、`qiniu`、`cos`。
