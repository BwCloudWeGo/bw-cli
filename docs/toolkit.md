# 工具组件总览与调用流程

这份文档列出当前脚手架内置的公共工具包，并按“配置 -> 初始化 -> 调用 -> 关闭或释放”的真实流程说明怎么用。业务项目通过 `bw-cli new` 生成后，这些包会直接进入新项目；其他 Go 项目也可以通过 `go get` 单独引入。

## 1. 公共工具列表

| 包 | 能力 | 典型使用位置 |
| --- | --- | --- |
| `pkg/config` | 本地 YAML / Nacos 配置加载和默认值 | `cmd/*/main.go` |
| `pkg/logger` | Zap 结构化日志、按日期命名、Lumberjack 轮转 | 所有进程入口 |
| `pkg/errors` | 业务错误码、HTTP/gRPC 状态映射 | `model/service/handler` |
| `pkg/httpx` | HTTP 统一响应结构 | Gin handler |
| `pkg/middleware` | CORS、JWT、RequestID、HTTP 请求日志 | Gateway router |
| `pkg/grpcx` | gRPC request_id 透传、服务端/客户端日志拦截器 | gRPC server/client |
| `pkg/database` | 根据配置打开 SQLite/MySQL/PostgreSQL Gorm | `cmd/*/main.go` |
| `pkg/mysqlx` | MySQL Gorm 初始化和连接池 | 独立 MySQL 项目 |
| `pkg/postgresx` | PostgreSQL Gorm 初始化和连接池 | 独立 PostgreSQL 项目 |
| `pkg/mongox` | MongoDB 官方 driver 初始化、Ping、Database 获取、公共 DocumentStore CRUD 操作 | `repo` 层 |
| `pkg/redisx` | Redis client 初始化和 Ping | 缓存、分布式锁、限流 |
| `pkg/esx` | Elasticsearch client 初始化 | 搜索、索引同步 |
| `pkg/kafkax` | Kafka reader/writer 初始化 | 事件发布和消费 |
| `pkg/filex` | 文件上传校验、对象 key 生成、MinIO/OSS/Qiniu/COS 上传 | `service` 或 `handler` |
| `pkg/alipayx` | 支付宝支付、同步回调验签、异步通知解析、退款封装 | `service` 或支付 handler |
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
go get github.com/BwCloudWeGo/bw-cli/pkg/alipayx
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

1. 默认把业务配置写在 `configs/config.yaml`。
2. 如需启用 Nacos，先把完整 `configs/config.yaml` 复制到 Nacos，再打开 `configs/nacos.yaml` 的 `enabled`。
3. 进程启动时调用 `config.InitGlobal`，由框架按开关选择本地 YAML 或 Nacos。
4. 通过 `config.MustGlobal()` 获取全局配置，再传给日志、数据库、中间件、文件上传等初始化函数。

示例：

```go
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()
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

JWT 密钥必须通过 `configs/config.yaml` 的 `middleware.jwt.secret` 提供。


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
    cfg.ServiceTarget("user"),
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

独立使用 MySQL：

```go
cfg := mysqlx.DefaultConfig()
cfg.DSN = ""
db, err := mysqlx.Open(cfg)
```

独立使用 PostgreSQL：

```go
cfg := postgresx.DefaultConfig()
cfg.DSN = ""
db, err := postgresx.Open(cfg)
```

## 9. MongoDB：`pkg/mongox`

调用流程：

1. 在当前配置来源中配置 `mongodb.*`。默认来源是 `configs/config.yaml`，启用 Nacos 后来源是 Nacos 中的完整业务 YAML。
2. 进程启动时调用 `mongox.NewClient`。
3. 调用 `mongox.Ping` 验证连接。
4. 使用 `mongox.Database` 获取业务数据库。
5. 在 `repo` 层为文档结构实现 `MongoCollectionName()`，再使用 `mongox.NewDocumentStore[T]` 封装 collection CRUD。
6. 进程退出时调用 `Disconnect`。

示例：

```go
type NoteDocument struct {
    ID    string `bson:"_id"`
    Title string `bson:"title"`
}

func (NoteDocument) MongoCollectionName() string {
    return "notes"
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
notes := mongox.NewDocumentStore[NoteDocument](db)

_, err = notes.UpsertByID(context.Background(), "note-1", &NoteDocument{
    ID:    "note-1",
    Title: "MongoDB note",
})
if err != nil {
    panic(err)
}

note, err := notes.FindByID(context.Background(), "note-1")
```

脚手架内调用示例见 [MongoDB 调用示例全流程](mongo-call-examples.md)。

## 10. Redis：`pkg/redisx`

调用流程：

1. 从配置读取 Redis 地址、账号、密码和 DB。
2. 调用 `redisx.NewClient`。
3. 启动时调用 `redisx.Ping`。
4. 业务里使用返回的 `*redis.Client` 做缓存、计数、限流等操作。
5. 需要跨实例互斥时，通过 `redisx.NewLocker` 创建分布式锁。
6. 进程退出时调用 `Close`。

缓存调用示例：

```go
client := redisx.NewClient(cfg.Redis)
defer client.Close()

if err := redisx.Ping(context.Background(), client); err != nil {
    panic(err)
}

err := client.Set(context.Background(), "cache:user:1", "value", time.Minute).Err()
```

分布式锁调用示例：

```go
locker := redisx.NewLocker(client, cfg.Redis.Lock)

lock, err := locker.Acquire(ctx, "jobs:daily-report", 30*time.Second)
if errors.Is(err, redisx.ErrLockNotAcquired) {
    return nil
}
if err != nil {
    return err
}
defer lock.Release(ctx)

// 只有拿到锁的实例会执行这里的业务逻辑。
return runDailyReport(ctx)
```

锁的释放使用 token 校验和 Lua 脚本，只有持有者才能释放自己的锁。长任务可以在业务执行中续租：

```go
if err := lock.Refresh(ctx, 30*time.Second); err != nil {
    return err
}
```

## 11. Elasticsearch：`pkg/esx`

调用流程：

1. 在 `configs/config.yaml` 配置 `elasticsearch.addresses`；用户名、密码、`cloud_id` 和 `api_key` 保留为可选认证参数。
2. 通过 `config.Load` 或 `config.MustGlobal()` 读取 YAML 后，调用 `esx.NewClient(cfg.Elasticsearch)` 创建官方 v7 client。
3. 调用 `esx.NewSearcherFromClient` 使用模糊搜索、高亮和聚合封装。
4. 复杂接口仍可直接使用官方 client。

初始化：

```go
cfg, err := config.Load("configs/config.yaml")
if err != nil {
    panic(err)
}
client, err := esx.NewClient(cfg.Elasticsearch)
if err != nil {
    panic(err)
}
searcher := esx.NewSearcherFromClient(client)
```

模糊搜索和高亮：

```go
result, err := searcher.FuzzySearch(ctx, esx.FuzzySearchRequest{
    Index:   "notes",
    Keyword: "golang",
    Fields:  []string{"title^2", "content"},
    Size:    20,
    Filters: []esx.Filter{
        esx.TermFilter("status", "published"),
    },
    Highlight: esx.HighlightConfig{
        Fields:   []string{"title", "content"},
        PreTags:  []string{"<mark>"},
        PostTags: []string{"</mark>"},
    },
})
if err != nil {
    return err
}
for _, hit := range result.Hits {
    fmt.Println(hit.ID, hit.Highlight["title"])
}
```

聚合查询：

```go
result, err := searcher.Aggregate(ctx, esx.AggregationRequest{
    Index: "notes",
    Aggregations: map[string]esx.Aggregation{
        "by_author": esx.TermsAggregation("author_id", 10),
        "by_day":    esx.DateHistogramAggregation("created_at", "day"),
    },
})
if err != nil {
    return err
}
fmt.Println(string(result.Aggregations))
```

MySQL 同步 ES 的增量扫描、bulk 写入和删除同步示例见 [Elasticsearch 调用示例](elasticsearch.md)。

## 12. Kafka：`pkg/kafkax`

生产者调用流程适合直接放在业务 service 中：

1. 配置 brokers 和 topic。
2. 调用 `kafkax.NewProducer`。
3. 使用 `Publish` 发布事件，topic 默认使用 `cfg.Kafka.Topic`。
4. 进程退出时关闭 producer。

进程入口初始化一次 producer，然后注入业务 service：

```go
producer, err := kafkax.NewProducer(cfg.Kafka)
if err != nil {
    return err
}
defer producer.Close()

noteSvc := noteservice.NewService(repo, producer, log)
```

业务 service 直接发布事件：

```go
func (s *Service) PublishNoteCreated(ctx context.Context, noteID string, authorID string) error {
    payload, err := json.Marshal(map[string]string{
        "event":     "note.created",
        "note_id":   noteID,
        "author_id": authorID,
    })
    if err != nil {
        return err
    }

    return s.kafka.Publish(ctx, kafkax.Message{
        Key:   noteID,
        Value: payload,
        Headers: map[string]string{
            "event": "note.created",
        },
    })
}
```

`kafkax.NewProducer(cfg.Kafka)` 创建的 writer 已经绑定了 `cfg.Kafka.Topic`，业务侧不要再给 `kafkax.Message.Topic` 赋值。即使误传，`Producer.Publish` 也会忽略该字段，避免 `kafka.(*Writer): Topic must not be specified for both Writer and Message`。

如果出现：

```text
[3] Unknown Topic Or Partition: the request is for a topic or partition that does not exist on this broker
```

说明 broker 上没有 `cfg.Kafka.Topic` 对应的 topic，且当前配置未允许自动创建。生产环境建议提前创建 topic：

```bash
kafka-topics.sh --bootstrap-server 127.0.0.1:9092 \
  --create \
  --topic xiaolanshu-events \
  --partitions 3 \
  --replication-factor 1
```

开发环境也可以临时打开：

```yaml
    kafka:
  producer:
    allow_auto_topic_creation: true
```

如果确实需要运行时指定 topic，不要用已绑定 topic 的 `NewProducer(cfg.Kafka)`，改用原生 writer 且不要设置 `Writer.Topic`：

```go
writer := &kafka.Writer{
    Addr: kafka.TCP(cfg.Kafka.Brokers...),
}
producer := kafkax.NewProducerWithWriter(writer)
defer producer.Close()

err := producer.Publish(ctx, kafkax.Message{
    Topic: "audit-events",
    Key:   "note-created",
    Value: payload,
    Headers: map[string]string{
        "event": "note.created",
    },
})
```

消费者调用流程：

```go
consumer, err := kafkax.NewConsumer(cfg.Kafka)
if err != nil {
    return err
}
defer consumer.Close()

err = consumer.Run(ctx, func(ctx context.Context, msg kafka.Message) error {
    // handler 返回 nil 后才会提交 offset；返回 error 会跳过提交，便于重试。
    return handleEvent(ctx, msg.Key, msg.Value)
})
if err != nil && !errors.Is(err, context.Canceled) {
    return err
}
```

如果业务需要直接使用 `segmentio/kafka-go` 的完整能力，可以继续使用兼容保留的原生入口：

```go
writer := kafkax.NewWriter(cfg.Kafka)
reader := kafkax.NewReader(cfg.Kafka)
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


如果使用自定义 bucket 域名：


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

## 14. 支付宝支付：`pkg/alipayx`

`pkg/alipayx` 基于 `github.com/smartwalle/alipay/v3` 封装常见支付链路：

```text
PagePay  -> 电脑网站支付跳转 URL
WapPay   -> 手机网站支付跳转 URL
AppPay   -> App SDK order string
VerifyReturn -> 同步 return_url 验签
DecodeNotification -> 异步 notify_url 验签并解析
Refund   -> 同步退款
```

### 14.1 配置

普通公钥模式：

```yaml
alipay:
  app_id: ""
  private_key: ""
  alipay_public_key: ""
  production: false
  notify_url: "https://api.example.com/payments/alipay/notify"
  return_url: "https://www.example.com/orders/alipay/return"
  encrypt_key: ""
```

证书模式：

```yaml
alipay:
  app_id: ""
  private_key: ""
  production: true
  notify_url: "https://api.example.com/payments/alipay/notify"
  return_url: "https://www.example.com/orders/alipay/return"
  app_cert_public_key_path: "configs/certs/appCertPublicKey.crt"
  alipay_root_cert_path: "configs/certs/alipayRootCert.crt"
  alipay_cert_public_key_path: "configs/certs/alipayCertPublicKey_RSA2.crt"
```


普通公钥模式和证书模式二选一，不要同时配置 `alipay_public_key` 和证书路径。

### 14.2 初始化

```go
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()

payClient, err := alipayx.NewClient(cfg.Alipay)
if err != nil {
    panic(err)
}

orderSvc := orderservice.NewService(repo, payClient, log)
```

建议在进程入口初始化一次，然后注入支付 service：

```text
handler -> service -> alipayx.Client
```

### 14.3 创建支付

实际业务 service 中直接调用 `pkg/alipayx`，不要在业务项目里再复制一层支付宝工具类。

```go
func (s *Service) CreateAlipayPagePayment(ctx context.Context, orderID string) (string, error) {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return "", err
    }
    if order.PaidAt != nil {
        return "", apperrors.FailedPrecondition("ORDER_ALREADY_PAID", "order already paid")
    }

    return s.alipay.PagePayURL(alipayx.PayRequest{
        OutTradeNo:     order.PayNo,
        Subject:        "小蓝书订单",
        TotalAmount:    order.PayAmount.StringFixed(2),
        Body:           "订单支付",
        TimeoutExpress: "15m",
    })
}

func (s *Service) CreateAlipayAppPayment(ctx context.Context, orderID string) (string, error) {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return "", err
    }

    return s.alipay.AppPay(alipayx.PayRequest{
        OutTradeNo:  order.PayNo,
        Subject:     "小蓝书订单",
        TotalAmount: order.PayAmount.StringFixed(2),
    })
}
```

Gin handler 只处理协议入参和出参：

```go
func CreateAlipayPagePayment(svc *orderservice.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            OrderID string `json:"order_id" binding:"required"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("INVALID_REQUEST", err.Error()))
            return
        }

        payURL, err := svc.CreateAlipayPagePayment(c.Request.Context(), req.OrderID)
        if err != nil {
            httpx.Error(c, err)
            return
        }
        httpx.OK(c, gin.H{"pay_url": payURL})
    }
}
```

### 14.4 同步回调验签

同步回调来自 `return_url`，适合展示支付结果页。它不能作为最终到账依据，最终状态以异步通知或主动查询为准。

```go
func AlipayReturn(payClient *alipayx.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        if err := c.Request.ParseForm(); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("INVALID_REQUEST", err.Error()))
            return
        }
        if err := payClient.VerifyReturn(c.Request.Context(), c.Request.Form); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("ALIPAY_SIGN_INVALID", err.Error()))
            return
        }

        httpx.OK(c, gin.H{
            "out_trade_no": c.Request.Form.Get("out_trade_no"),
            "trade_no":     c.Request.Form.Get("trade_no"),
        })
    }
}
```

### 14.5 异步通知回调

支付宝异步通知需要验签、校验订单号和金额、幂等更新订单状态，处理成功后返回纯文本 `success`。

```go
func AlipayNotify(payClient *alipayx.Client, svc *orderservice.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        if err := c.Request.ParseForm(); err != nil {
            c.String(http.StatusBadRequest, "fail")
            return
        }

        notification, err := payClient.DecodeNotification(c.Request.Context(), c.Request.Form)
        if err != nil {
            c.String(http.StatusBadRequest, "fail")
            return
        }

        err = svc.MarkAlipayPaid(
            c.Request.Context(),
            notification.OutTradeNo,
            notification.TradeNo,
            notification.TotalAmount,
            notification.TradeStatus,
        )
        if err != nil {
            c.String(http.StatusInternalServerError, "fail")
            return
        }

        c.String(http.StatusOK, "success")
    }
}
```

`MarkPaid` 里建议做这些业务校验：

1. `OutTradeNo` 必须存在且属于本系统订单。
2. `TotalAmount` 必须等于订单应付金额。
3. `TradeStatus` 只在 `TRADE_SUCCESS` 或 `TRADE_FINISHED` 时标记已支付。
4. 已处理过的通知直接返回成功，避免支付宝重试造成重复入账。

### 14.6 退款

```go
func (s *Service) RefundAlipayOrder(ctx context.Context, orderID string, reason string) error {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return err
    }
    if order.RefundedAt != nil {
        return nil
    }

    return s.alipay.RefundOK(ctx, alipayx.RefundRequest{
        OutTradeNo:   order.PayNo,
        RefundAmount: order.PayAmount.StringFixed(2),
        RefundReason: reason,
        OutRequestNo: order.PayNo + "-refund-001",
    })
}
```

多次部分退款时，`OutRequestNo` 必须为每次退款请求生成唯一值。

## 15. Validator：`pkg/validator`

当前提供轻量的通用函数：

```go
if !validator.Required(req.Email, req.Password) {
    return errors.InvalidArgument("INVALID_ARGUMENT", "email and password are required")
}
```

复杂参数校验建议在 request DTO 上结合 `gin.ShouldBindJSON` 和 `binding` tag，业务规则仍放在 `model/service`。

## 16. 脚手架生成：`pkg/scaffold`

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

## 17. 推荐启动顺序

在生成项目后，推荐按这个顺序把工具串起来：

1. `config.InitGlobal` 读取配置并写入 `config.GlobalConfig`。
2. `logger.WithDailyFileName` 和 `logger.New` 初始化日志。
3. `database.Open` 或各数据源独立初始化。
4. `filex.NewUploader` 初始化文件上传接口。
5. `alipayx.NewClient` 初始化支付宝支付接口。
6. 初始化 repo，把 DB、MongoDB、Redis、ES、Kafka、Uploader、Alipay client 注入业务服务。
7. 初始化 gRPC server 或 Gin router。
8. 注册 middleware 和 interceptor。
9. 启动服务。

这个顺序能保证配置项都来自系统配置，公共能力只在入口初始化一次，业务层拿到的是稳定接口，后续替换数据库或云存储 provider 时改配置即可。
