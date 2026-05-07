# MongoDB 调用示例全流程

这份文档专门说明脚手架项目中如何调用 `pkg/mongox` 公共 MongoDB 封装。重点是业务服务如何通过配置文件、启动入口、repo 层和公共 `DocumentStore` 完成真实读写。

本文关注真实业务接入流程：

- 配置文件如何写。
- 服务启动时如何初始化 MongoDB client。
- `repo` 层如何封装集合操作。
- `service` 层如何调用仓储接口。
- 不同服务如何复用同一套 `mongox.DocumentStore[T]`。
- 如何写单元测试和本地联调。

## 1. 调用规则

MongoDB 调用必须遵守脚手架分层：

```text
handler -> service -> model.Repository -> repo -> pkg/mongox -> MongoDB
```

各层职责：

| 层级 | 是否可以直接调用 MongoDB | 应该做什么 |
| --- | --- | --- |
| `cmd/<service>` | 可以初始化 client | 读取配置、创建 MongoDB client、Ping、选择 database、注入 repo |
| `handler` | 不可以 | 只把 gRPC/HTTP 请求转换成 `dto.Command` |
| `dto` | 不可以 | 只定义业务入参和业务出参 |
| `service` | 不可以直接调 driver 或 `mongox.DocumentStore` | 编排业务流程，只依赖 `model.Repository` |
| `model` | 不可以 | 定义领域实体、业务错误、仓储接口 |
| `repo` | 可以 | 定义 MongoDB 文档结构，调用 `mongox.DocumentStore[T]` 做 CRUD |
| `pkg/mongox` | 可以 | 公共 MongoDB client 和 collection 操作封装 |

这样做的目的很简单：业务逻辑不和 MongoDB driver 耦合，后续切换 MySQL、PostgreSQL、MongoDB 或写 fake repository 都更轻。

## 2. 配置 MongoDB

在 `configs/config.yaml` 中配置：

```yaml
mongodb:
  uri: mongodb://127.0.0.1:27017
  username: ""
  password: ""
  database: xiaolanshu
  app_name: bw-cli
  min_pool_size: 0
  max_pool_size: 100
  connect_timeout_seconds: 10
  server_selection_timeout_seconds: 5
```

脚手架默认从配置文件读取 MongoDB 连接信息。启用账号密码时，直接填写 `username`、`password`、`database` 等配置项即可；服务启动后会通过 `cfg.MongoDB.MongoxConfig()` 统一转换成 `mongox.Config`。

配置读取链路：

```text
configs/config.yaml
  -> pkg/config.Load
  -> config.InitGlobal
  -> config.MustGlobal()
  -> cfg.MongoDB.MongoxConfig()
  -> mongox.NewClient(...)
```

## 3. 服务启动时初始化 MongoDB

每个独立 gRPC 服务进程都应该创建自己的 MongoDB client。一个进程只创建一个 client，退出时关闭。

示例：`cmd/note/main.go`

```go
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()

log, err := logger.New(cfg.Log)
if err != nil {
    panic(err)
}
defer log.Sync()

mongoClient, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
if err != nil {
    log.Fatal("create mongodb client failed", zap.Error(err))
}
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := mongoClient.Disconnect(ctx); err != nil {
        log.Error("disconnect mongodb failed", zap.Error(err))
    }
}()

if err := mongox.Ping(context.Background(), mongoClient); err != nil {
    log.Fatal("ping mongodb failed", zap.Error(err))
}

mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)
repo := noterepo.NewMongoRepository(mongoDB, log)
svc := noteservice.NewService(repo)
server := notehandler.NewServer(svc, log)
```

如果一个服务需要同时访问多个 collection，不要创建多个 client。应该复用同一个 `mongoDB`：

```go
mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)

noteRepo := noterepo.NewMongoRepository(mongoDB, log)
commentRepo := commentrepo.NewMongoRepository(mongoDB, log)
auditRepo := auditrepo.NewMongoRepository(mongoDB, log)
```

## 4. 公共操作类怎么用

`mongox.NewDocumentStore[T]` 是业务仓储推荐使用的公共 MongoDB 操作类入口。`T` 是集合文档结构体，通常定义在业务服务的 `repo` 包中，并通过 `MongoCollectionName()` 声明集合名称。

```go
type NoteDocument struct {
    ID        string    `bson:"_id"`
    AuthorID  string    `bson:"author_id"`
    Title     string    `bson:"title"`
    Content   string    `bson:"content"`
    CreatedAt time.Time `bson:"created_at"`
    UpdatedAt time.Time `bson:"updated_at"`
}

func (NoteDocument) MongoCollectionName() string {
    return "notes"
}

notes := mongox.NewDocumentStore[NoteDocument](mongoDB, log)
```

目前公共操作类支持：

| 方法 | 用途 |
| --- | --- |
| `Insert` | 新增单条文档 |
| `UpsertByID` | 按 `_id` 保存，存在则替换，不存在则新增 |
| `ReplaceByID` | 按 `_id` 替换已有文档，不默认 upsert |
| `ReplaceOne` | 按自定义 filter 替换文档 |
| `FindByID` | 按 `_id` 查询单条文档 |
| `FindOne` | 按自定义 filter 查询单条文档 |
| `FindMany` | 按 filter 查询多条文档，支持排序和分页 |
| `UpdateOne` | 局部更新单条文档 |
| `DeleteByID` | 按 `_id` 删除 |
| `DeleteOne` | 按自定义 filter 删除 |
| `Count` | 统计文档数量 |

公共类会自动记录结构化日志字段：

```text
datasource=mongodb
collection=<collection>
operation=<Insert/FindOne/UpdateOne/...>
latency_ms=<耗时>
matched_count / modified_count / deleted_count / count
error=<错误>
```

## 5. 示例一：note 服务保存笔记到 MongoDB

当前仓库的 note 服务已经接入 MongoDB，完整链路如下：

```text
cmd/note/main.go
  -> mongox.NewClient(cfg.MongoDB.MongoxConfig())
  -> mongoDB := mongox.Database(client, cfg.MongoDB.Database)
  -> internal/note/repo.NewMongoRepository(mongoDB, log)
  -> internal/note/service.NewService(repo)
  -> internal/note/handler.NewServer(svc, log)
```

### 5.1 model 层定义仓储接口

`internal/note/model/repository.go`

```go
package model

import "context"

type Repository interface {
    Save(ctx context.Context, note *Note) error
    FindByID(ctx context.Context, id string) (*Note, error)
}
```

`service` 只依赖这个接口，不知道底层是 MongoDB：

```go
type Service struct {
    repo model.Repository
}

func (s *Service) Create(ctx context.Context, cmd dto.CreateNoteCommand) (*dto.NoteDTO, error) {
    note, err := model.NewNote(cmd.AuthorID, cmd.Title, cmd.Content)
    if err != nil {
        return nil, err
    }
    if err := s.repo.Save(ctx, note); err != nil {
        return nil, err
    }
    return dto.FromNote(note), nil
}
```

### 5.2 repo 层定义 MongoDB 文档

`internal/note/repo/mongo_repository.go`

```go
type NoteDocument struct {
    ID          string     `bson:"_id"`
    AuthorID    string     `bson:"author_id"`
    Title       string     `bson:"title"`
    Content     string     `bson:"content"`
    Status      int32      `bson:"status"`
    NoteType    int32      `bson:"note_type"`
    Permission  int32      `bson:"permission"`
    Remark      string     `bson:"remark"`
    TopicIDs    []string   `bson:"topic_ids"`
    PublishedAt *time.Time `bson:"published_at,omitempty"`
    CreatedAt   time.Time  `bson:"created_at"`
    UpdatedAt   time.Time  `bson:"updated_at"`
}
```

注意：`bson` tag 只放在 repo 层文档结构里，不要放到 `model.Note` 上。

### 5.3 repo 层调用公共 Mongo 类

```go
const noteCollectionName = "notes"

func (NoteDocument) MongoCollectionName() string {
    return noteCollectionName
}

type MongoRepository struct {
    notes mongox.DocumentSaverFinder[NoteDocument]
    log   *zap.Logger
}

func NewMongoRepository(db *mongo.Database, loggers ...*zap.Logger) *MongoRepository {
    log := optionalLogger(loggers...)
    store := mongox.NewDocumentStore[NoteDocument](db, log)
    return NewMongoRepositoryWithStore(store, log)
}

func NewMongoRepositoryWithStore(store mongox.DocumentSaverFinder[NoteDocument], loggers ...*zap.Logger) *MongoRepository {
    return &MongoRepository{notes: store, log: optionalLogger(loggers...)}
}

func (r *MongoRepository) Save(ctx context.Context, note *model.Note) error {
    _, err := r.notes.UpsertByID(ctx, note.ID, toNoteDocument(note))
    return err
}
```

### 5.4 查询时转换错误

公共类找不到文档时返回 `mongox.ErrNotFound`。repo 层要把它转换成业务领域错误：

```go
func (r *MongoRepository) FindByID(ctx context.Context, id string) (*model.Note, error) {
    document, err := r.notes.FindByID(ctx, id)
    if errors.Is(err, mongox.ErrNotFound) {
        return nil, model.ErrNoteNotFound
    }
    if err != nil {
        return nil, err
    }
    return toNoteFromDocument(document), nil
}
```

## 6. 示例二：order 服务改成 MongoDB CRUD

通过 `bw-cli service order --tidy` 生成的服务默认是 Gorm 仓储。如果某个服务更适合文档存储，可以保留 `model.Repository` 接口不变，只新增一个 MongoDB 仓储实现。

### 6.1 保持 model 接口不变

`internal/order/model/repository.go`

```go
type Repository interface {
    Save(ctx context.Context, item *Order) error
    FindByID(ctx context.Context, id string) (*Order, error)
    List(ctx context.Context, offset int, limit int) ([]*Order, int64, error)
    Delete(ctx context.Context, id string) error
}
```

### 6.2 新增 MongoDB 仓储

新增文件：`internal/order/repo/mongo_repository.go`

```go
package repo

import (
    "context"
    "errors"
    "time"

    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
    "go.uber.org/zap"

    "github.com/BwCloudWeGo/bw-cli/internal/order/model"
    "github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

const orderCollectionName = "orders"

type OrderDocument struct {
    ID          string    `bson:"_id"`
    Name        string    `bson:"name"`
    Description string    `bson:"description"`
    CreatedAt   time.Time `bson:"created_at"`
    UpdatedAt   time.Time `bson:"updated_at"`
}

func (OrderDocument) MongoCollectionName() string {
    return orderCollectionName
}

type MongoRepository struct {
    orders *mongox.DocumentStore[OrderDocument]
    log    *zap.Logger
}

func NewMongoRepository(db *mongo.Database, log *zap.Logger) *MongoRepository {
    if log == nil {
        log = zap.NewNop()
    }
    return &MongoRepository{
        orders: mongox.NewDocumentStore[OrderDocument](db, log),
        log:    log,
    }
}

func (r *MongoRepository) Save(ctx context.Context, item *model.Order) error {
    _, err := r.orders.UpsertByID(ctx, item.ID, toOrderDocument(item))
    return err
}

func (r *MongoRepository) FindByID(ctx context.Context, id string) (*model.Order, error) {
    document, err := r.orders.FindByID(ctx, id)
    if errors.Is(err, mongox.ErrNotFound) {
        return nil, model.ErrOrderNotFound
    }
    if err != nil {
        return nil, err
    }
    return toOrderDomain(document), nil
}

func (r *MongoRepository) List(ctx context.Context, offset int, limit int) ([]*model.Order, int64, error) {
    filter := bson.M{}
    total, err := r.orders.Count(ctx, filter)
    if err != nil {
        return nil, 0, err
    }

    documents, err := r.orders.FindMany(ctx, filter,
        options.Find().
            SetSort(bson.D{{Key: "created_at", Value: -1}}).
            SetSkip(int64(offset)).
            SetLimit(int64(limit)),
    )
    if err != nil {
        return nil, 0, err
    }

    items := make([]*model.Order, 0, len(documents))
    for i := range documents {
        items = append(items, toOrderDomain(&documents[i]))
    }
    return items, total, nil
}

func (r *MongoRepository) Delete(ctx context.Context, id string) error {
    result, err := r.orders.DeleteByID(ctx, id)
    if err != nil {
        return err
    }
    if result.DeletedCount == 0 {
        return model.ErrOrderNotFound
    }
    return nil
}

func toOrderDocument(item *model.Order) *OrderDocument {
    return &OrderDocument{
        ID:          item.ID,
        Name:        item.Name,
        Description: item.Description,
        CreatedAt:   item.CreatedAt,
        UpdatedAt:   item.UpdatedAt,
    }
}

func toOrderDomain(document *OrderDocument) *model.Order {
    return &model.Order{
        ID:          document.ID,
        Name:        document.Name,
        Description: document.Description,
        CreatedAt:   document.CreatedAt,
        UpdatedAt:   document.UpdatedAt,
    }
}

var _ model.Repository = (*MongoRepository)(nil)
```

这个示例里，`service` 和 `handler` 不需要改。只要 `MongoRepository` 实现了 `model.Repository`，业务层就可以直接复用。

### 6.3 在 order main 中注入 MongoDB 仓储

`cmd/order/main.go` 中把 Gorm repo 替换为 Mongo repo：

```go
mongoClient, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
if err != nil {
    log.Fatal("create mongodb client failed", zap.Error(err))
}
defer disconnectMongo(mongoClient, log)

if err := mongox.Ping(context.Background(), mongoClient); err != nil {
    log.Fatal("ping mongodb failed", zap.Error(err))
}

mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)
repo := orderrepo.NewMongoRepository(mongoDB, log)
svc := orderservice.NewService(repo, log)
```

如果只想让 order 服务使用 MongoDB，其他服务继续使用 Gorm，也没有问题。每个服务入口自己决定注入哪个 repository。

## 7. 示例三：audit 服务写操作日志

操作日志、审计日志、行为事件很适合放 MongoDB。假设新增一个 `audit` 服务，用 MongoDB 追加写入审计事件。

### 7.1 文档结构

`internal/audit/repo/mongo_repository.go`

```go
type AuditEventDocument struct {
    ID         string         `bson:"_id"`
    Service    string         `bson:"service"`
    Action     string         `bson:"action"`
    OperatorID string         `bson:"operator_id"`
    ResourceID string         `bson:"resource_id"`
    Metadata   map[string]any `bson:"metadata"`
    CreatedAt  time.Time      `bson:"created_at"`
}

func (AuditEventDocument) MongoCollectionName() string {
    return "audit_events"
}
```

### 7.2 写入审计事件

```go
type MongoRepository struct {
    events *mongox.DocumentStore[AuditEventDocument]
}

func NewMongoRepository(db *mongo.Database, log *zap.Logger) *MongoRepository {
    return &MongoRepository{
        events: mongox.NewDocumentStore[AuditEventDocument](db, log),
    }
}

func (r *MongoRepository) SaveEvent(ctx context.Context, event *model.AuditEvent) error {
    document := &AuditEventDocument{
        ID:         event.ID,
        Service:    event.Service,
        Action:     event.Action,
        OperatorID: event.OperatorID,
        ResourceID: event.ResourceID,
        Metadata:   event.Metadata,
        CreatedAt:  event.CreatedAt,
    }
    _, err := r.events.Insert(ctx, document)
    return err
}
```

### 7.3 查询某个资源的审计日志

```go
func (r *MongoRepository) ListByResource(ctx context.Context, resourceID string, limit int64) ([]*model.AuditEvent, error) {
    documents, err := r.events.FindMany(ctx,
        bson.M{"resource_id": resourceID},
        options.Find().
            SetSort(bson.D{{Key: "created_at", Value: -1}}).
            SetLimit(limit),
    )
    if err != nil {
        return nil, err
    }

    events := make([]*model.AuditEvent, 0, len(documents))
    for i := range documents {
        events = append(events, toAuditEventDomain(&documents[i]))
    }
    return events, nil
}
```

这里使用 `Insert`，是因为审计事件通常是追加写，不应该覆盖旧数据。

## 8. 多服务调用 MongoDB 的推荐方式

### 8.1 每个服务独立进程

微服务部署时，`user-service`、`note-service`、`order-service` 通常是独立进程。每个进程读取同一套 `mongodb.*` 配置，并创建自己的 client：

```text
cmd/note/main.go  -> note Mongo client  -> notes collection
cmd/order/main.go -> order Mongo client -> orders collection
cmd/audit/main.go -> audit Mongo client -> audit_events collection
```

这种方式最清晰，服务之间互不共享内存连接，部署和扩容也独立。

### 8.2 单进程内多个 repo

如果某个服务内部要同时写多个集合，可以在 `cmd/<service>/main.go` 中只创建一次 client，然后注入多个 repo：

```go
mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)

noteRepo := noterepo.NewMongoRepository(mongoDB, log)
auditRepo := auditrepo.NewMongoRepository(mongoDB, log)

svc := noteservice.NewService(noteRepo, auditRepo, log)
```

### 8.3 不同服务使用不同 database

如果你希望不同服务隔离数据库，可以在配置文件中为服务增加独立配置字段，或为不同部署环境准备不同的 `configs/config.yaml`。服务代码仍然只从 `cfg.MongoDB.Database` 读取数据库名。

代码仍然不变：

```go
mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)
```

## 9. 索引在哪里创建

CRUD 优先使用 `mongox.DocumentStore[T]`。索引属于数据库结构初始化，可以放在 repo 包中单独写函数，然后在 `cmd/<service>/main.go` 启动时调用。

示例：

```go
func EnsureNoteIndexes(ctx context.Context, db *mongo.Database) error {
    _, err := db.Collection("notes").Indexes().CreateMany(ctx, []mongo.IndexModel{
        {
            Keys: bson.D{
                {Key: "author_id", Value: 1},
                {Key: "created_at", Value: -1},
            },
        },
        {
            Keys: bson.D{
                {Key: "status", Value: 1},
                {Key: "published_at", Value: -1},
            },
        },
    })
    return err
}
```

启动时调用：

```go
if err := noterepo.EnsureNoteIndexes(context.Background(), mongoDB); err != nil {
    log.Fatal("ensure note mongodb indexes failed", zap.Error(err))
}
```

不要在每次请求里创建索引。

## 10. 单元测试怎么写

不要在 service 单测里连真实 MongoDB。service 单测使用 fake repository；repo 单测使用 fake `mongox.DocumentSaverFinder[NoteDocument]` 或者只测转换逻辑。

note 仓储当前已经用了公共小接口隔离 MongoDB 操作：

```go
type DocumentSaverFinder[T any] interface {
    UpsertByID(ctx context.Context, id any, document *T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error)
    FindByID(ctx context.Context, id any, opts ...options.Lister[options.FindOneOptions]) (*T, error)
}
```

测试时注入 fake：

```go
store := &fakeNoteDocumentStore{}
repository := repo.NewMongoRepositoryWithStore(store)

note, _ := model.NewNote("user-1", "Mongo note", "content")
err := repository.Save(context.Background(), note)

require.NoError(t, err)
require.Equal(t, note.ID, store.upsertID)
require.Equal(t, note.Title, store.upsertDocument.Title)
```

公共操作类本身在 `pkg/mongox/collection_test.go` 和 `pkg/mongox/document_store_test.go` 中覆盖：

- `FindByID` 能正确解码文档。
- 查不到时转换成 `mongox.ErrNotFound`。
- `UpsertByID` 会自动带 `upsert=true`。
- `FindMany` 能解码多条文档。
- 写入错误会原样返回给业务仓储。
- `DocumentStore` 会把 `Insert`、`UpsertByID`、`FindByID`、`FindMany`、`UpdateOne`、`DeleteByID`、`Count` 等操作完整转发到底层集合。

## 11. 本地真实 MongoDB 联调

默认测试不会连接真实 MongoDB。note 服务提供了一个显式开关的联调示例：

```bash
export APP_RUN_NOTE_MONGODB_EXAMPLE=true

go test ./cmd/note -run TestRunMongoDocumentStoreExampleUsesCurrentConfig -v
```

运行前请先确认 `configs/config.yaml` 中的 `mongodb.*` 可以连接到本地或测试环境 MongoDB。这个测试会读取当前配置，调用 `cmd/note/mongodb_example.go`，对 `note_mongodb_examples` 集合执行：

1. `UpsertByID` 保存示例文档。
2. `FindByID` 查询示例文档。
3. `UpdateOne` 局部更新文档。
4. `Count` 统计当前服务示例文档数量。
5. 再次 `FindByID` 返回最终文档。

如果没有设置 `APP_RUN_NOTE_MONGODB_EXAMPLE=true`，测试会自动跳过，不影响普通 `go test ./...`。

## 12. 常见错误

### 12.1 在 handler 里直接调用 MongoDB

错误写法：

```go
func (h *Handler) Create(c *gin.Context) {
    collection := h.mongo.Database("app").Collection("notes")
    collection.InsertOne(c, document)
}
```

问题：

- handler 和数据库耦合，无法独立测试。
- 业务规则容易散落在控制器里。
- 后续替换存储方式会牵动 HTTP/gRPC 层。

正确做法：

```text
handler 组装 dto.Command
service 编排业务
repo 调用 mongox.DocumentStore
```

### 12.2 在 service 里直接 new DocumentStore

错误写法：

```go
func (s *Service) Create(ctx context.Context, cmd dto.CreateCommand) error {
    notes := mongox.NewDocumentStore[NoteDocument](s.mongoDB)
    _, err := notes.Insert(ctx, document)
    return err
}
```

正确做法：把 `mongox.NewDocumentStore` 放进 `repo`，service 只依赖 `model.Repository`。

### 12.3 把 bson tag 写到领域模型上

错误写法：

```go
type Note struct {
    ID string `bson:"_id"`
}
```

正确做法：领域模型保持纯净，MongoDB 文档结构放在 `repo` 包。

### 12.4 忘记处理 ErrNotFound

`mongox.DocumentStore` 查询不到时返回 `mongox.ErrNotFound`。repo 层要转换成业务错误：

```go
if errors.Is(err, mongox.ErrNotFound) {
    return nil, model.ErrNoteNotFound
}
```

handler 再把业务错误转换成 HTTP/gRPC 错误码。

## 13. 推荐落地清单

接入一个新服务时按这个顺序做：

1. 在 `configs/config.yaml` 配置 `mongodb.*`。
2. 在 `cmd/<service>/main.go` 中创建 MongoDB client、Ping、获取 database。
3. 在 `internal/<service>/model/repository.go` 定义业务需要的仓储接口。
4. 在 `internal/<service>/repo/mongo_repository.go` 定义 MongoDB 文档结构。
5. 在文档结构体上实现 `MongoCollectionName() string`，声明集合名称。
6. 在 repo 中调用 `mongox.NewDocumentStore[T](db, log)`。
7. 在 repo 中完成领域模型和文档模型转换。
8. 在 repo 中把 `mongox.ErrNotFound` 转成领域错误。
9. 在 `service` 中继续依赖 `model.Repository`，不要直接依赖 MongoDB。
10. 在 `handler` 中只做请求转换和错误映射。
11. 为 repo 写 fake 单测，为真实 MongoDB 写显式开关的集成测试。

做到这 11 步，MongoDB 就能在不同业务服务中稳定复用，而且不会破坏脚手架的 DDD 分层。
