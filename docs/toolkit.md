# 工具组件总览与调用流程

这份文档列出当前脚手架内置的公共工具包，并按“配置 -> 初始化 -> 调用 -> 关闭或释放”的真实流程说明怎么用。业务项目通过 `bw-cli new` 生成后，这些包会直接进入新项目；其他 Go 项目也可以通过 `go get` 单独引入。

## 1. 公共工具列表

| 包 | 能力 | 典型使用位置 |
| --- | --- | --- |
| `pkg/config` | YAML 配置加载、环境变量覆盖、默认值 | `cmd/*/main.go` |
| `pkg/logger` | Zap 结构化日志、按日期命名、Lumberjack 轮转 | 所有进程入口 |
| `pkg/errors` | 业务错误码、HTTP/gRPC 状态映射 | `model/service/handler` |
| `pkg/httpx` | HTTP 统一响应结构 | Gin handler |
| `pkg/middleware` | CORS、JWT、RequestID、HTTP 请求日志 | Gateway router |
| `pkg/grpcx` | gRPC request_id 透传、服务端/客户端日志拦截器 | gRPC server/client |
| `pkg/database` | 根据配置打开 SQLite/MySQL/PostgreSQL Gorm | `cmd/*/main.go` |
| `pkg/mysqlx` | MySQL Gorm 初始化和连接池 | 独立 MySQL 项目 |
| `pkg/postgresx` | PostgreSQL Gorm 初始化和连接池 | 独立 PostgreSQL 项目 |
| `pkg/mongox` | MongoDB 官方 driver 初始化、Ping、Database 获取、公共 Collection CRUD 操作 | `repo` 层 |
| `pkg/redisx` | Redis client 初始化和 Ping | 缓存、分布式锁、限流 |
| `pkg/esx` | Elasticsearch client 初始化 | 搜索、索引同步 |
| `pkg/kafkax` | Kafka reader/writer 初始化 | 事件发布和消费 |
| `pkg/filex` | 文件上传校验、对象 key 生成、MinIO/OSS/Qiniu/COS 上传 | `service` 或 `handler` |
| `pkg/validator` | 简单通用校验函数 | DTO 或业务入参校验 |
| `pkg/scaffold` | `bw-cli new` 项目生成逻辑 | CLI 内部 |
| `pkg/observability` | 可观测性注册占位入口 | 进程启动 |

## 2. 安装和引用

安装脚手架命令：

```bash
go install github.com/BwCloudWeGo/bw-cli/cmd/bw-cli@latest
```

在业务项目里单独引用公共包：

```bash
go get github.com/BwCloudWeGo/bw-cli/pkg/config
go get github.com/BwCloudWeGo/bw-cli/pkg/logger
go get github.com/BwCloudWeGo/bw-cli/pkg/database
go get github.com/BwCloudWeGo/bw-cli/pkg/mongox
go get github.com/BwCloudWeGo/bw-cli/pkg/filex
```

生成完整项目：

```bash
bw-cli new test_cli \
  --module github.com/your-org/test_cli \
  --tidy
```

`bw-cli new` 默认生成不带业务 demo 的干净框架；需要 user/note 演示项目时使用：

```bash
bw-cli demo demo_cli \
  --module github.com/your-org/demo_cli \
  --tidy
```

生成后进入项目：

```bash
cd test_cli
make tools
make proto
make test
```

## 3. 配置加载：`pkg/config`

调用流程：

1. 在 `configs/config.yaml` 写入默认配置。
2. 用 `APP_` 前缀环境变量覆盖敏感值或环境差异值。
3. 进程启动时调用 `config.InitGlobal`。
4. 通过 `config.MustGlobal()` 获取全局配置，再传给日志、数据库、中间件、文件上传等初始化函数。

示例：

```go
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()
```

环境变量覆盖规则：

```text
APP_ + 配置路径大写 + 下划线
```

示例：

```bash
export APP_HTTP_PORT=8081
export APP_DATABASE_DRIVER=postgres
export APP_FILE_STORAGE_PROVIDER=minio
```

## 4. 日志：`pkg/logger`

调用流程：

1. 从 `cfg.Log` 读取日志配置。
2. 使用 `logger.WithDailyFileName` 把日志文件名改成 `服务名-日期.log`。
3. 调用 `logger.New` 创建 Zap logger。
4. 在 HTTP/gRPC/业务/仓储层通过字段记录详细维度。

示例：

```go
logCfg := logger.WithDailyFileName(cfg.Log, time.Now())
log, err := logger.New(logCfg)
if err != nil {
    panic(err)
}
defer log.Sync()
```

默认文件策略：

```yaml
log:
  file:
    enabled: true
    max_size_mb: 128
    max_backups: 14
    max_age_days: 7
    compress: true
```

日志会记录的主要维度：

- HTTP：method、path、route、status、client_ip、latency_ms、request_bytes、response_bytes、request_id。
- gRPC：full_method、peer、status_code、latency_ms、request_id。
- 业务：service、env、use_case、user_id、aggregate_id、error_code。
- 仓储：repository、operation、rows_affected、latency_ms、error。

## 5. HTTP 中间件：`pkg/middleware`

调用流程：

1. Gateway 启动时创建 Gin engine。
2. 注册 `RequestID`，保证每个请求都有 `X-Request-ID`。
3. 注册 `RequestLogger`，输出 HTTP 请求日志。
4. 注册 `CORS`。
5. 对需要鉴权的路由组注册 `JWTAuth`。

示例：

```go
r := gin.New()
r.Use(middleware.RequestID())
r.Use(middleware.RequestLogger(log))
r.Use(middleware.CORS(cfg.Middleware.CORS))

auth := r.Group("/api/v1")
auth.Use(middleware.JWTAuth(cfg.Middleware.JWT))
```

生成 JWT：

```go
token, err := middleware.GenerateToken(cfg.Middleware.JWT, middleware.JWTClaims{
    UserID: "user-id-from-database",
    Role:   "admin",
})
```

读取 JWT claims：

```go
claims := middleware.ClaimsFromContext(c)
```

JWT 密钥必须通过配置或环境变量提供：

```bash
export APP_MIDDLEWARE_JWT_SECRET='replace-with-real-secret'
```

## 6. HTTP 响应和错误：`pkg/httpx`、`pkg/errors`

调用流程：

1. `model/service` 返回 `errors.AppError` 或普通 error。
2. `handler` 统一调用 `httpx.OK`、`httpx.Created`、`httpx.Error`。
3. `httpx.Error` 根据错误类型输出 HTTP 状态码和业务错误码。

示例：

```go
user, err := h.userClient.GetUser(c.Request.Context(), req)
if err != nil {
    httpx.Error(c, err)
    return
}
httpx.OK(c, user)
```

业务错误示例：

```go
return errors.NotFound("USER_NOT_FOUND", "user not found")
```

## 7. gRPC 拦截器：`pkg/grpcx`

服务端调用流程：

1. 创建 gRPC server。
2. 注册 `grpcx.UnaryServerInterceptor(log)`。
3. handler 返回 `errors.ToGRPC(err)`，让错误码跨协议传递。

示例：

```go
server := grpc.NewServer(
    grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)),
)
```

客户端调用流程：

```go
conn, err := grpc.DialContext(
    ctx,
    cfg.GRPC.UserTarget,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithUnaryInterceptor(grpcx.UnaryClientInterceptor(requestID)),
)
```

## 8. Gorm 数据库：`pkg/database`、`pkg/mysqlx`、`pkg/postgresx`

统一入口调用流程：

1. 配置 `database.driver`。
2. 配置对应数据库 DSN 和连接池。
3. 在服务进程入口调用 `database.Open`。
4. 把 `*gorm.DB` 注入 repo。

示例：

```go
db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
if err != nil {
    log.Fatal("open database failed", zap.Error(err))
}
```

支持的 `database.driver`：

```text
sqlite
mysql
postgres
postgresql
pg
```

MySQL 环境变量示例：

```bash
export APP_DATABASE_DRIVER=mysql
export APP_MYSQL_DSN='user:password@tcp(mysql.example.com:3306)/app?charset=utf8mb4&parseTime=True&loc=Local'
```

PostgreSQL 环境变量示例：

```bash
export APP_DATABASE_DRIVER=postgres
export APP_POSTGRESQL_DSN='host=postgres.example.com user=app password=replace-with-real-password dbname=app port=5432 sslmode=require TimeZone=Asia/Shanghai'
```

独立使用 MySQL：

```go
cfg := mysqlx.DefaultConfig()
cfg.DSN = os.Getenv("APP_MYSQL_DSN")
db, err := mysqlx.Open(cfg)
```

独立使用 PostgreSQL：

```go
cfg := postgresx.DefaultConfig()
cfg.DSN = os.Getenv("APP_POSTGRESQL_DSN")
db, err := postgresx.Open(cfg)
```

## 9. MongoDB：`pkg/mongox`

调用流程：

1. 在 `configs/config.yaml` 中配置 `mongodb.*`。
2. 进程启动时调用 `mongox.NewClient`。
3. 调用 `mongox.Ping` 验证连接。
4. 使用 `mongox.Database` 获取业务数据库。
5. 在 `repo` 层使用 `mongox.NewCollection[T]` 封装 collection CRUD。
6. 进程退出时调用 `Disconnect`。

示例：

```go
type NoteDocument struct {
    ID    string `bson:"_id"`
    Title string `bson:"title"`
}

if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()

client, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
if err != nil {
    panic(err)
}
defer client.Disconnect(context.Background())

if err := mongox.Ping(context.Background(), client); err != nil {
    panic(err)
}

db := mongox.Database(client, cfg.MongoDB.Database)
notes := mongox.NewCollection[NoteDocument](db, "notes")

_, err = notes.UpsertByID(context.Background(), "note-1", &NoteDocument{
    ID:    "note-1",
    Title: "MongoDB note",
})
if err != nil {
    panic(err)
}

note, err := notes.FindByID(context.Background(), "note-1")
```

脚手架内调用示例见 [MongoDB 调用示例全流程](mongo-call-examples.md)，基础教学见 [MongoDB 从 0 到 1 教学教程](mongodb.md)。

## 10. Redis：`pkg/redisx`

调用流程：

1. 从配置读取 Redis 地址、账号、密码和 DB。
2. 调用 `redisx.NewClient`。
3. 启动时调用 `redisx.Ping`。
4. 业务里使用返回的 `*redis.Client`。
5. 进程退出时调用 `Close`。

示例：

```go
client := redisx.NewClient(cfg.Redis)
defer client.Close()

if err := redisx.Ping(context.Background(), client); err != nil {
    panic(err)
}

err := client.Set(context.Background(), "cache:user:1", "value", time.Minute).Err()
```

## 11. Elasticsearch：`pkg/esx`

调用流程：

1. 配置 `elasticsearch.addresses`、用户名和密码。
2. 调用 `esx.NewClient`。
3. 在 repo 或索引同步组件中使用官方 client。

示例：

```go
client, err := esx.NewClient(cfg.Elasticsearch)
if err != nil {
    panic(err)
}

res, err := client.Info()
if err != nil {
    panic(err)
}
defer res.Body.Close()
```

## 12. Kafka：`pkg/kafkax`

生产者调用流程：

1. 配置 brokers 和 topic。
2. 调用 `kafkax.NewWriter`。
3. 使用 `WriteMessages` 发布事件。
4. 进程退出时关闭 writer。

示例：

```go
writer := kafkax.NewWriter(cfg.Kafka)
defer writer.Close()

err := writer.WriteMessages(ctx, kafka.Message{
    Key:   []byte("note-created"),
    Value: []byte(`{"note_id":"note-id-from-business"}`),
})
```

消费者调用流程：

```go
reader := kafkax.NewReader(cfg.Kafka)
defer reader.Close()

msg, err := reader.ReadMessage(ctx)
if err != nil {
    return err
}
_ = msg
```

## 13. 文件上传：`pkg/filex`

`pkg/filex` 提供统一上传接口，业务代码不直接依赖具体云厂商 SDK。当前支持：

```text
minio
oss
qiniu
cos
```

默认支持的文件类型：

- Word：`.doc`、`.docx`
- PDF：`.pdf`
- 图片：`.jpg`、`.jpeg`、`.png`、`.gif`、`.webp`、`.bmp`、`.svg`
- 视频：`.mp4`、`.mov`、`.avi`、`.mkv`、`.webm`
- 音频：`.mp3`、`.wav`、`.ogg`、`.m4a`、`.flac`、`.aac`

默认最大文件大小：

```text
100 MB
```

### 13.1 通用配置

```yaml
file_storage:
  provider: minio
  max_size_mb: 100
  object_prefix: uploads
  public_base_url: ""
  allowed_extensions:
    - .doc
    - .docx
    - .pdf
    - .jpg
    - .jpeg
    - .png
    - .gif
    - .webp
    - .bmp
    - .svg
    - .mp4
    - .mov
    - .avi
    - .mkv
    - .webm
    - .mp3
    - .wav
    - .ogg
    - .m4a
    - .flac
    - .aac
```

常用环境变量：

```bash
export APP_FILE_STORAGE_PROVIDER=minio
export APP_FILE_STORAGE_MAX_SIZE_MB=100
export APP_FILE_STORAGE_OBJECT_PREFIX=uploads
export APP_FILE_STORAGE_PUBLIC_BASE_URL='https://cdn.example.com'
```

### 13.2 MinIO 配置

```yaml
file_storage:
  provider: minio
  minio:
    endpoint: ""
    access_key_id: ""
    secret_access_key: ""
    bucket: ""
    region: ""
    use_ssl: false
```

环境变量：

```bash
export APP_FILE_STORAGE_PROVIDER=minio
export APP_FILE_STORAGE_MINIO_ENDPOINT='127.0.0.1:9000'
export APP_FILE_STORAGE_MINIO_ACCESS_KEY_ID='replace-with-real-access-key'
export APP_FILE_STORAGE_MINIO_SECRET_ACCESS_KEY='replace-with-real-secret-key'
export APP_FILE_STORAGE_MINIO_BUCKET='app-files'
export APP_FILE_STORAGE_MINIO_USE_SSL=false
```

### 13.3 阿里云 OSS 配置

```yaml
file_storage:
  provider: oss
  oss:
    endpoint: ""
    access_key_id: ""
    access_key_secret: ""
    bucket: ""
```

环境变量：

```bash
export APP_FILE_STORAGE_PROVIDER=oss
export APP_FILE_STORAGE_OSS_ENDPOINT='https://oss-cn-hangzhou.aliyuncs.com'
export APP_FILE_STORAGE_OSS_ACCESS_KEY_ID='replace-with-real-access-key'
export APP_FILE_STORAGE_OSS_ACCESS_KEY_SECRET='replace-with-real-secret-key'
export APP_FILE_STORAGE_OSS_BUCKET='app-files'
```

### 13.4 七牛云 Kodo 配置

```yaml
file_storage:
  provider: qiniu
  qiniu:
    access_key: ""
    secret_key: ""
    bucket: ""
    region: ""
    use_https: true
    use_cdn_domains: false
```

环境变量：

```bash
export APP_FILE_STORAGE_PROVIDER=qiniu
export APP_FILE_STORAGE_QINIU_ACCESS_KEY='replace-with-real-access-key'
export APP_FILE_STORAGE_QINIU_SECRET_KEY='replace-with-real-secret-key'
export APP_FILE_STORAGE_QINIU_BUCKET='app-files'
export APP_FILE_STORAGE_QINIU_REGION='z0'
export APP_FILE_STORAGE_QINIU_USE_HTTPS=true
```

### 13.5 腾讯云 COS 配置

```yaml
file_storage:
  provider: cos
  cos:
    secret_id: ""
    secret_key: ""
    bucket: ""
    region: ""
    bucket_url: ""
```

环境变量：

```bash
export APP_FILE_STORAGE_PROVIDER=cos
export APP_FILE_STORAGE_COS_SECRET_ID='replace-with-real-secret-id'
export APP_FILE_STORAGE_COS_SECRET_KEY='replace-with-real-secret-key'
export APP_FILE_STORAGE_COS_BUCKET='app-files-1250000000'
export APP_FILE_STORAGE_COS_REGION='ap-guangzhou'
```

如果使用自定义 bucket 域名：

```bash
export APP_FILE_STORAGE_COS_BUCKET_URL='https://app-files-1250000000.cos.ap-guangzhou.myqcloud.com'
```

### 13.6 直接调用上传接口

初始化：

```go
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()

uploader, err := filex.NewUploader(cfg.FileStorage)
if err != nil {
    panic(err)
}
```

Gin handler 示例：

```go
// import apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"

func UploadFile(uploader filex.Uploader) gin.HandlerFunc {
    return func(c *gin.Context) {
        file, header, err := c.Request.FormFile("file")
        if err != nil {
            httpx.Error(c, apperrors.InvalidArgument("FILE_REQUIRED", "file is required"))
            return
        }
        defer file.Close()

        result, err := uploader.Upload(c.Request.Context(), filex.UploadRequest{
            Reader:      file,
            Filename:    header.Filename,
            ContentType: header.Header.Get("Content-Type"),
            Size:        header.Size,
            Metadata: map[string]string{
                "uploaded_by": "user-id-from-auth-context",
            },
        })
        if err != nil {
            httpx.Error(c, apperrors.InvalidArgument("UPLOAD_FAILED", err.Error()))
            return
        }

        httpx.Created(c, result)
    }
}
```

调用结果：

```json
{
  "provider": "minio",
  "bucket": "app-files",
  "key": "uploads/2026/04/29/uuid.pdf",
  "url": "https://cdn.example.com/uploads/2026/04/29/uuid.pdf",
  "etag": "...",
  "size": 1024,
  "content_type": "application/pdf"
}
```

上传时会自动做这些事情：

1. 校验文件名不能为空。
2. 校验文件大小必须大于 0。
3. 校验文件大小不能超过 `file_storage.max_size_mb`。
4. 校验扩展名必须在 `allowed_extensions` 中。
5. 校验 MIME 必须在 `allowed_content_types` 中。
6. 未传 `ObjectKey` 时生成 `object_prefix/YYYY/MM/DD/uuid.ext`。
7. 根据 `provider` 调用对应云存储 SDK。
8. 如果配置了 `public_base_url`，返回可访问 URL。

### 13.7 业务层推荐放置方式

对于正式业务，建议把 `filex.Uploader` 注入到 `service`：

```text
handler -> service -> filex.Uploader
```

这样 handler 只处理协议入参，service 负责“用户是否允许上传、上传后是否入库、是否发布事件”等业务编排。

## 14. Validator：`pkg/validator`

当前提供轻量的通用函数：

```go
if !validator.Required(req.Email, req.Password) {
    return errors.InvalidArgument("INVALID_ARGUMENT", "email and password are required")
}
```

复杂参数校验建议在 request DTO 上结合 `gin.ShouldBindJSON` 和 `binding` tag，业务规则仍放在 `model/service`。

## 15. 脚手架生成：`pkg/scaffold`

CLI 调用：

```bash
bw-cli new demo-app \
  --module github.com/your-org/demo-app \
  --tidy
```

内部流程：

1. 默认使用 `git clone --depth 1` 从官方仓库拉取脚手架。
2. 如果传了 `--source`，复制本地脚手架目录。
3. 跳过 `.git`、`logs`、`data`、`tmp` 等运行时目录。
4. 读取源项目 `go.mod` 的 module。
5. 替换 `.go`、`.mod`、`.md`、`.yaml`、`.yml`、`.proto` 中的 module 路径。
6. 跳过 `*.pb.go`，避免破坏 protobuf descriptor。
7. `new` 移除 user/note 示例业务和脚手架自身 CLI 代码。
8. `demo` 保留 user/note 示例业务，方便学习和演示。
9. 重写生成项目内的 README、usage、architecture、toolkit、mongodb 文档，让文档和实际目录保持一致。
10. 如果传了 `--tidy`，执行 `go mod tidy`。

代码调用：

```go
err := scaffold.Init(scaffold.InitOptions{
    SourceDir:  ".",
    TargetDir:  "../demo-app",
    ModulePath: "github.com/your-org/demo-app",
    RunTidy:    true,
})
```

## 16. 推荐启动顺序

在生成项目后，推荐按这个顺序把工具串起来：

1. `config.InitGlobal` 读取配置并写入 `config.GlobalConfig`。
2. `logger.WithDailyFileName` 和 `logger.New` 初始化日志。
3. `database.Open` 或各数据源独立初始化。
4. `filex.NewUploader` 初始化文件上传接口。
5. 初始化 repo，把 DB、MongoDB、Redis、ES、Kafka、Uploader 注入业务服务。
6. 初始化 gRPC server 或 Gin router。
7. 注册 middleware 和 interceptor。
8. 启动服务。

这个顺序能保证配置项都来自系统配置，公共能力只在入口初始化一次，业务层拿到的是稳定接口，后续替换数据库或云存储 provider 时改配置即可。
