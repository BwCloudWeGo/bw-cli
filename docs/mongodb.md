# MongoDB 从 0 到 1 教学教程

这份文档用于教学，不只是告诉你“配置什么参数”，而是按真实学习路径带你从零开始理解 MongoDB，并把它接入 `bw-cli` 生成的 Go 微服务项目。

如果你已经了解 MongoDB，只想看脚手架项目里如何调用 `pkg/mongox`、如何在 note/order/audit 等不同服务中落地，请直接看 [MongoDB 调用示例全流程](mongo-call-examples.md)。

学完后你应该能做到：

- 知道 MongoDB 和 MySQL/PostgreSQL 的核心区别。
- 能在本地启动 MongoDB，并用 `mongosh` 完成基础 CRUD。
- 能理解 database、collection、document、BSON、ObjectID、index 这些概念。
- 能在脚手架中读取 MongoDB 配置，创建 MongoDB client。
- 能按 DDD 分层把 MongoDB 代码放到 `repo` 层。
- 能写出基本的保存、查询、分页、索引和测试代码。
- 能排查常见连接、认证、索引和查询问题。

## 1. 先理解 MongoDB 是什么

MongoDB 是文档型数据库。它保存的不是“表中的一行”，而是一个个 JSON 风格的文档。MongoDB 实际存储格式是 BSON，可以理解成支持更多数据类型的二进制 JSON。

关系型数据库（例如 MySQL、PostgreSQL）常见结构：

```text
database
  └── table
      └── row
          └── column
```

MongoDB 常见结构：

```text
database
  └── collection
      └── document
          └── field
```

对照关系：

| 关系型数据库 | MongoDB | 说明 |
| --- | --- | --- |
| database | database | 数据库 |
| table | collection | 集合，类似表 |
| row | document | 文档，类似一行数据 |
| column | field | 字段 |
| primary key | `_id` | 每个文档默认都有 `_id` |
| index | index | 索引 |

一个 MongoDB 文档长这样：

```json
{
  "_id": "662f0a05ccf6b828ec622c1a",
  "note_id": "note-10001",
  "author_id": "user-1",
  "title": "MongoDB 入门",
  "content": "这是一篇笔记正文",
  "status": "draft",
  "tags": ["mongodb", "go"],
  "created_at": "2026-04-29T10:00:00Z",
  "updated_at": "2026-04-29T10:00:00Z"
}
```

## 2. 什么时候适合用 MongoDB

MongoDB 适合结构灵活、字段变化快、以聚合文档为核心的数据。

适合：

- 内容草稿、笔记正文、富文本 JSON。
- 用户扩展资料、偏好设置、第三方平台原始回调。
- 操作日志、行为事件、审计记录。
- 商品详情、文章详情、配置快照。
- 不同业务类型字段差异较大的数据。

不太适合：

- 强事务、强一致的账务流水。
- 复杂多表 join 的统计报表。
- 高度规范化的主业务关系模型。
- 必须依赖外键约束保证正确性的场景。

在这个脚手架里，推荐的实践是：

- 主业务库：MySQL 或 PostgreSQL，通过 Gorm 使用。
- 灵活文档库：MongoDB，通过官方 driver 使用。
- 缓存：Redis。
- 搜索：Elasticsearch。
- 消息：Kafka。

## 3. 准备环境

### 3.1 检查 Go

```bash
go version
```

本脚手架要求：

```text
go1.25+
```

### 3.2 检查 bw-cli 项目

如果你还没有生成项目，先安装脚手架命令：

```bash
go install github.com/BwCloudWeGo/bw-cli/cmd/bw-cli@latest
```

生成一个教学项目：

```bash
bw-cli new test_cli \
  --module github.com/BwCloudWeGo/test_cli \
  --tidy
```

这会生成一个不带 user/note demo 的干净项目。若你想基于演示项目学习完整 user/note 调用链，可以改用：

```bash
bw-cli demo test_cli_demo \
  --module github.com/BwCloudWeGo/test_cli_demo \
  --tidy
```

进入项目：

```bash
cd test_cli
```

## 4. 准备 MongoDB 连接

你只需要准备一个当前机器可以访问的 MongoDB 实例，然后把连接信息写入 `configs/config.yaml`：

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

如果 MongoDB 开启了账号密码，直接填写 `username` 和 `password`。如果需要副本集、认证库或其他连接参数，把完整连接串写在 `uri` 中。

## 5. 使用 mongosh 做第一次连接

使用配置文件中的连接信息执行 `mongosh`：

```bash
mongosh 'mongodb://127.0.0.1:27017/xiaolanshu'
```

连接后执行：

```javascript
db.runCommand({ ping: 1 })
```

看到类似结果说明连接正常：

```javascript
{ ok: 1 }
```

查看当前数据库：

```javascript
db.getName()
```

查看所有数据库：

```javascript
show dbs
```

切换数据库：

```javascript
use xiaolanshu
```

查看集合：

```javascript
show collections
```

## 6. 用 mongosh 学会基础 CRUD

这一节先不写 Go 代码，只用命令行理解 MongoDB 的基本操作。

### 6.1 插入一条文档

```javascript
db.note_documents.insertOne({
  note_id: "note-10001",
  author_id: "user-1",
  title: "MongoDB 入门",
  content: "这是第一篇 MongoDB 笔记",
  status: "draft",
  tags: ["mongodb", "go"],
  created_at: new Date(),
  updated_at: new Date()
})
```

说明：

- `note_documents` 是集合名。集合不存在时，MongoDB 会在第一次写入时创建。
- `insertOne` 插入一条文档。
- 没有显式传 `_id` 时，MongoDB 自动生成 `_id`。

### 6.2 查询全部文档

```javascript
db.note_documents.find()
```

格式化输出：

```javascript
db.note_documents.find().pretty()
```

### 6.3 按字段查询

```javascript
db.note_documents.find({ author_id: "user-1" })
```

只查一条：

```javascript
db.note_documents.findOne({ note_id: "note-10001" })
```

### 6.4 更新文档

```javascript
db.note_documents.updateOne(
  { note_id: "note-10001" },
  {
    $set: {
      title: "MongoDB 从 0 到 1",
      updated_at: new Date()
    }
  }
)
```

说明：

- 第一个参数是查询条件。
- 第二个参数是更新动作。
- `$set` 表示只更新指定字段，不覆盖整篇文档。

### 6.5 不存在则插入

```javascript
db.note_documents.updateOne(
  { note_id: "note-10002" },
  {
    $set: {
      author_id: "user-1",
      title: "第二篇笔记",
      content: "upsert 示例",
      status: "draft",
      updated_at: new Date()
    },
    $setOnInsert: {
      created_at: new Date()
    }
  },
  { upsert: true }
)
```

`upsert` 的意思是：

- 查到了就更新。
- 没查到就插入。

### 6.6 删除文档

```javascript
db.note_documents.deleteOne({ note_id: "note-10002" })
```

### 6.7 统计数量

```javascript
db.note_documents.countDocuments({ author_id: "user-1" })
```

## 7. 理解索引

如果没有索引，MongoDB 查询大量文档时可能会全集合扫描。生产环境中，所有高频查询都应该配索引。

### 7.1 创建唯一索引

业务上 `note_id` 不允许重复，可以创建唯一索引：

```javascript
db.note_documents.createIndex(
  { note_id: 1 },
  { name: "idx_note_id", unique: true }
)
```

### 7.2 创建组合索引

如果页面经常按作者查询，并按更新时间倒序排列：

```javascript
db.note_documents.createIndex(
  { author_id: 1, updated_at: -1 },
  { name: "idx_author_updated_at" }
)
```

说明：

- `1` 表示升序。
- `-1` 表示降序。
- 组合索引字段顺序很重要。

### 7.3 查看索引

```javascript
db.note_documents.getIndexes()
```

### 7.4 看查询是否命中索引

```javascript
db.note_documents.find({ author_id: "user-1" })
  .sort({ updated_at: -1 })
  .explain("executionStats")
```

重点看：

- `totalDocsExamined`：扫描了多少文档。
- `totalKeysExamined`：扫描了多少索引键。
- `executionTimeMillis`：耗时。

如果 `totalDocsExamined` 很大，通常说明索引不合适。

## 8. 脚手架中的 MongoDB 配置

项目配置在 `configs/config.yaml`：

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

字段说明：

| 字段 | 说明 |
| --- | --- |
| `uri` | MongoDB 连接串 |
| `username` | MongoDB 用户名。为空时不单独设置认证信息 |
| `password` | MongoDB 密码。启用认证时直接通过配置文件读取 |
| `database` | 默认业务数据库 |
| `app_name` | 客户端名称，便于监控识别 |
| `min_pool_size` | 最小连接池数量 |
| `max_pool_size` | 最大连接池数量 |
| `connect_timeout_seconds` | TCP 建连超时 |
| `server_selection_timeout_seconds` | 选择可用节点超时 |

服务启动时会读取这组配置，并通过 `cfg.MongoDB.MongoxConfig()` 转换成公共连接配置。不同环境需要不同 MongoDB 地址时，维护对应环境的配置文件即可。

## 9. 脚手架内置的 mongox 包

脚手架提供了 `pkg/mongox`：

```text
pkg/mongox
  ├── collection.go
  ├── collection_test.go
  ├── mongox.go
  └── mongox_test.go
```

它做四件事：

- 定义 MongoDB 连接配置。
- 创建官方 driver 的 `mongo.Client`。
- 提供 `Ping` 方法验证连接。
- 提供公共 `Collection[T]` 操作类，封装常见 CRUD，业务仓储不需要散落调用 driver。

核心用法：

```go
client, err := mongox.NewClient(mongox.Config{
    URI:                    cfg.MongoDB.URI,
    Username:               cfg.MongoDB.Username,
    Password:               cfg.MongoDB.Password,
    Database:               cfg.MongoDB.Database,
    AppName:                cfg.MongoDB.AppName,
    MinPoolSize:            cfg.MongoDB.MinPoolSize,
    MaxPoolSize:            cfg.MongoDB.MaxPoolSize,
    ConnectTimeout:         cfg.MongoDB.ConnectTimeout(),
    ServerSelectionTimeout: cfg.MongoDB.ServerSelectionTimeout(),
})
```

注意：`mongo.Client` 是连接池对象，一个进程创建一次即可。不要每个请求创建一次。

公共操作类用法：

```go
type NoteDocument struct {
    ID      string `bson:"_id"`
    Title   string `bson:"title"`
    Content string `bson:"content"`
}

db := mongox.Database(client, cfg.MongoDB.Database)
notes := mongox.NewCollection[NoteDocument](db, "notes")

_, err = notes.UpsertByID(ctx, "note-1", &NoteDocument{
    ID:      "note-1",
    Title:   "MongoDB note",
    Content: "stored by mongox.Collection",
})
if err != nil {
    return err
}

note, err := notes.FindByID(ctx, "note-1")
if err != nil {
    return err
}
```

`Collection[T]` 当前提供：

| 方法 | 作用 |
| --- | --- |
| `Insert` | 插入单条文档 |
| `UpsertByID` | 按 `_id` 替换保存，不存在则新增 |
| `ReplaceByID` / `ReplaceOne` | 替换已有文档 |
| `FindByID` / `FindOne` | 查询单条文档，未找到返回 `mongox.ErrNotFound` |
| `FindMany` | 查询多条文档，支持 `options.Find()` |
| `UpdateOne` | 执行 `$set`、`$inc` 等局部更新 |
| `DeleteByID` / `DeleteOne` | 删除文档 |
| `Count` | 统计文档数量 |

## 10. 在服务启动时创建 MongoDB client

示例：在 `cmd/note/main.go` 中创建 MongoDB client。

先补 import：

```go
import (
    "context"
    "time"

    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.uber.org/zap"

    "github.com/BwCloudWeGo/bw-cli/pkg/config"
    "github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)
```

创建 helper：

```go
func openMongo(cfg *config.Config, log *zap.Logger) *mongo.Client {
    client, err := mongox.NewClient(mongox.Config{
        URI:                    cfg.MongoDB.URI,
        Username:               cfg.MongoDB.Username,
        Password:               cfg.MongoDB.Password,
        Database:               cfg.MongoDB.Database,
        AppName:                cfg.MongoDB.AppName,
        MinPoolSize:            cfg.MongoDB.MinPoolSize,
        MaxPoolSize:            cfg.MongoDB.MaxPoolSize,
        ConnectTimeout:         cfg.MongoDB.ConnectTimeout(),
        ServerSelectionTimeout: cfg.MongoDB.ServerSelectionTimeout(),
    })
    if err != nil {
        log.Fatal("create mongodb client failed", zap.Error(err))
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := mongox.Ping(ctx, client); err != nil {
        log.Fatal("ping mongodb failed", zap.Error(err))
    }
    return client
}
```

在 `main` 中使用：

```go
mongoClient := openMongo(cfg, log)
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := mongoClient.Disconnect(ctx); err != nil {
        log.Error("disconnect mongodb failed", zap.Error(err))
    }
}()
```

note 服务还提供了一个可手动运行的 MongoDB 操作示例：

```bash
APP_RUN_NOTE_MONGODB_EXAMPLE=true go test ./cmd/note -run TestRunMongoCollectionExampleUsesCurrentConfig -v
```

这个示例会读取当前仓库的 `configs/config.yaml`，使用其中的 `mongodb.uri`、`mongodb.username`、`mongodb.password` 和 `mongodb.database` 创建客户端，然后通过 `mongox.NewCollection[T]` 对 `note_mongodb_examples` 集合执行：

1. `UpsertByID` 写入或替换示例文档。
2. `FindByID` 按 `_id` 查询示例文档。
3. `UpdateOne` 局部更新示例文档。
4. `Count` 统计当前 note 服务的示例文档数量。

示例代码位于 `cmd/note/mongodb_example.go`。默认测试不会连接真实 MongoDB，只有显式设置 `APP_RUN_NOTE_MONGODB_EXAMPLE=true` 才会执行数据写入。

## 11. DDD 中 MongoDB 代码应该放在哪里

脚手架分层：

```text
internal/<service>
  ├── model
  ├── service
  ├── repo
  └── handler
```

推荐放法：

```text
internal/note
  ├── model
  │   ├── note.go
  │   └── note_document.go
  ├── service
  ├── repo
  │   ├── gorm_repository.go
  │   └── mongo_repository.go
  └── handler
```

规则：

- `model` 定义领域对象和仓储接口。
- `service` 编排业务流程。
- `repo` 实现 MongoDB 读写。
- `handler` 不直接操作 MongoDB。

不要在 `handler` 里写：

```go
collection.Find(ctx, bson.M{"author_id": id})
```

这种写法会让控制器和数据库细节耦合。应该让 `handler` 调 `service`，`service` 调接口，`repo` 负责 MongoDB 细节。

## 12. 定义文档模型

示例：给笔记服务增加 MongoDB 文档模型。

文件：`internal/note/model/note_document.go`

```go
package model

import (
    "context"
    "time"
)

type NoteDocument struct {
    ID        string
    NoteID    string
    AuthorID  string
    Title     string
    Content   string
    Status    string
    Tags      []string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type NoteDocumentRepository interface {
    EnsureIndexes(ctx context.Context) error
    Save(ctx context.Context, doc NoteDocument) error
    FindByNoteID(ctx context.Context, noteID string) (NoteDocument, error)
    ListByAuthor(ctx context.Context, authorID string, limit int64) ([]NoteDocument, error)
    DeleteByNoteID(ctx context.Context, noteID string) error
}
```

这里的 `model.NoteDocument` 不引入 MongoDB SDK。它是领域层可以理解的数据结构。

## 13. 在 repo 层定义 BSON 结构

文件：`internal/note/repo/mongo_repository.go`

```go
package repo

import (
    "context"
    "time"

    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"

    "github.com/BwCloudWeGo/bw-cli/internal/note/model"
    "github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

type noteDocumentRecord struct {
    ID        bson.ObjectID `bson:"_id,omitempty"`
    NoteID    string        `bson:"note_id"`
    AuthorID  string        `bson:"author_id"`
    Title     string        `bson:"title"`
    Content   string        `bson:"content"`
    Status    string        `bson:"status"`
    Tags      []string      `bson:"tags"`
    CreatedAt time.Time     `bson:"created_at"`
    UpdatedAt time.Time     `bson:"updated_at"`
}

type NoteMongoRepository struct {
    collection *mongo.Collection
    documents  *mongox.Collection[noteDocumentRecord]
}

func NewNoteMongoRepository(client *mongo.Client, database string) *NoteMongoRepository {
    db := mongox.Database(client, database)
    return &NoteMongoRepository{
        collection: db.Collection("note_documents"),
        documents:  mongox.NewCollection[noteDocumentRecord](db, "note_documents"),
    }
}
```

为什么领域模型和 BSON 模型分开？

- 领域层不依赖 MongoDB SDK。
- BSON tag 只出现在基础设施层。
- 后续替换存储方案时，业务层不需要跟着改。

## 14. 实现模型转换

继续写在 `internal/note/repo/mongo_repository.go`：

```go
func toNoteDocumentRecord(doc model.NoteDocument) noteDocumentRecord {
    record := noteDocumentRecord{
        NoteID:    doc.NoteID,
        AuthorID:  doc.AuthorID,
        Title:     doc.Title,
        Content:   doc.Content,
        Status:    doc.Status,
        Tags:      doc.Tags,
        CreatedAt: doc.CreatedAt,
        UpdatedAt: doc.UpdatedAt,
    }
    if doc.ID != "" {
        if objectID, err := bson.ObjectIDFromHex(doc.ID); err == nil {
            record.ID = objectID
        }
    }
    return record
}

func toNoteDocument(record noteDocumentRecord) model.NoteDocument {
    return model.NoteDocument{
        ID:        record.ID.Hex(),
        NoteID:    record.NoteID,
        AuthorID:  record.AuthorID,
        Title:     record.Title,
        Content:   record.Content,
        Status:    record.Status,
        Tags:      record.Tags,
        CreatedAt: record.CreatedAt,
        UpdatedAt: record.UpdatedAt,
    }
}
```

## 15. 创建索引

```go
func (r *NoteMongoRepository) EnsureIndexes(ctx context.Context) error {
    _, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
        {
            Keys:    bson.D{{Key: "note_id", Value: 1}},
            Options: options.Index().SetName("idx_note_id").SetUnique(true),
        },
        {
            Keys:    bson.D{{Key: "author_id", Value: 1}, {Key: "updated_at", Value: -1}},
            Options: options.Index().SetName("idx_author_updated_at"),
        },
        {
            Keys:    bson.D{{Key: "tags", Value: 1}},
            Options: options.Index().SetName("idx_tags"),
        },
    })
    return err
}
```

什么时候调用？

在服务启动时调用一次：

```go
noteDocRepo := repo.NewNoteMongoRepository(mongoClient, cfg.MongoDB.Database)
if err := noteDocRepo.EnsureIndexes(context.Background()); err != nil {
    log.Fatal("ensure mongodb indexes failed", zap.Error(err))
}
```

生产环境中，超大集合创建索引可能影响性能，应该由发布脚本或 DBA 流程提前执行。教学和小项目可以在启动时创建。

## 16. 保存文档

```go
func (r *NoteMongoRepository) Save(ctx context.Context, doc model.NoteDocument) error {
    now := time.Now()
    if doc.CreatedAt.IsZero() {
        doc.CreatedAt = now
    }
    doc.UpdatedAt = now

    record := toNoteDocumentRecord(doc)
    set := bson.M{
        "note_id":    record.NoteID,
        "author_id":  record.AuthorID,
        "title":      record.Title,
        "content":    record.Content,
        "status":     record.Status,
        "tags":       record.Tags,
        "updated_at": record.UpdatedAt,
    }

    _, err := r.documents.UpdateOne(
        ctx,
        bson.M{"note_id": record.NoteID},
        bson.M{
            "$set": set,
            "$setOnInsert": bson.M{
                "_id":        bson.NewObjectID(),
                "created_at": record.CreatedAt,
            },
        },
        options.UpdateOne().SetUpsert(true),
    )
    return err
}
```

这里用 `UpdateOne + upsert`，原因是 `note_id` 是业务唯一键：

- 第一次保存：插入。
- 后续保存：更新。
- 不需要调用方判断文档是否存在。

## 17. 按 note_id 查询

```go
func (r *NoteMongoRepository) FindByNoteID(ctx context.Context, noteID string) (model.NoteDocument, error) {
    record, err := r.documents.FindOne(ctx, bson.M{"note_id": noteID})
    if err != nil {
        return model.NoteDocument{}, err
    }
    return toNoteDocument(*record), nil
}
```

如果没查到，公共操作类会返回 `mongox.ErrNotFound`。业务层可以把它转换成自己的业务错误：

```go
if errors.Is(err, mongox.ErrNotFound) {
    return model.NoteDocument{}, apperrors.NewNotFound("note document not found")
}
```

## 18. 按作者分页查询

简单分页：

```go
func (r *NoteMongoRepository) ListByAuthor(ctx context.Context, authorID string, limit int64) ([]model.NoteDocument, error) {
    if limit <= 0 || limit > 100 {
        limit = 20
    }

    records, err := r.documents.FindMany(
        ctx,
        bson.M{"author_id": authorID},
        options.Find().
            SetSort(bson.D{{Key: "updated_at", Value: -1}}).
            SetLimit(limit),
    )
    if err != nil {
        return nil, err
    }

    docs := make([]model.NoteDocument, 0, len(records))
    for _, record := range records {
        docs = append(docs, toNoteDocument(record))
    }
    return docs, nil
}
```

注意：

- `limit` 要有限制，避免一次查太多。
- `mongox.Collection.FindMany` 会负责 cursor 解码和关闭。
- 查询字段和排序字段要匹配索引。

## 19. 删除文档

```go
func (r *NoteMongoRepository) DeleteByNoteID(ctx context.Context, noteID string) error {
    _, err := r.documents.DeleteOne(ctx, bson.M{"note_id": noteID})
    return err
}
```

如果业务需要软删除，不要直接删除，可以更新状态：

```go
_, err := r.documents.UpdateOne(
    ctx,
    bson.M{"note_id": noteID},
    bson.M{"$set": bson.M{"status": "deleted", "updated_at": time.Now()}},
)
```

## 20. 在 service 层使用 MongoDB 仓储

`service` 层依赖接口，不依赖 MongoDB SDK：

```go
type NoteService struct {
    noteRepo     model.NoteRepository
    documentRepo model.NoteDocumentRepository
}

func NewService(noteRepo model.NoteRepository, documentRepo model.NoteDocumentRepository) *NoteService {
    return &NoteService{
        noteRepo:      noteRepo,
        documentRepo: documentRepo,
    }
}
```

保存笔记时，同时写关系型数据库和 MongoDB 文档库：

```go
func (s *NoteService) SaveDraft(ctx context.Context, note model.Note) error {
    if err := s.noteRepo.Save(ctx, note); err != nil {
        return err
    }
    return s.documentRepo.Save(ctx, model.NoteDocument{
        NoteID:    note.ID,
        AuthorID:  note.AuthorID,
        Title:     note.Title,
        Content:   note.Content,
        Status:    string(note.Status),
        CreatedAt: note.CreatedAt,
        UpdatedAt: note.UpdatedAt,
    })
}
```

教学时可以这样讲：

- 关系型数据库保存结构化主数据。
- MongoDB 保存适合文档读取的内容快照。
- 两者同时写入时要考虑失败补偿。简单项目可以先同步写，复杂项目建议通过事务消息或 Kafka 异步同步。

## 21. 大数据量分页：游标分页

`skip + limit` 简单，但页数越深越慢。大数据量建议用游标分页。

请求第一页：

```go
filter := bson.M{"author_id": authorID}
```

请求下一页时带上上一页最后一条的 `updated_at` 和 `_id`：

```go
filter := bson.M{
    "author_id": authorID,
    "$or": []bson.M{
        {"updated_at": bson.M{"$lt": lastUpdatedAt}},
        {"updated_at": lastUpdatedAt, "_id": bson.M{"$lt": lastID}},
    },
}
```

排序：

```go
sort := bson.D{
    {Key: "updated_at", Value: -1},
    {Key: "_id", Value: -1},
}
```

索引：

```javascript
db.note_documents.createIndex(
  { author_id: 1, updated_at: -1, _id: -1 },
  { name: "idx_author_updated_id" }
)
```

## 22. MongoDB 事务

MongoDB 单文档写入是原子的。多集合、多文档原子写入才需要事务。

事务前提：

- MongoDB 运行在副本集或分片集群中。
- 单机普通模式不支持事务。
- 事务回调可能被 driver 重试，所以回调里的逻辑要具备幂等性。

示例：

```go
session, err := client.StartSession()
if err != nil {
    return err
}
defer session.EndSession(ctx)

_, err = session.WithTransaction(ctx, func(txCtx context.Context) (any, error) {
    if err := repoA.Save(txCtx, docA); err != nil {
        return nil, err
    }
    if err := repoB.Save(txCtx, docB); err != nil {
        return nil, err
    }
    return nil, nil
})
return err
```

教学建议：初学阶段先掌握单文档原子更新和合理建模，不要一开始就依赖事务。

## 23. 写一个集成测试

MongoDB 集成测试依赖本地服务，不应该默认强制运行。推荐用显式开关控制。

文件：`cmd/note/mongodb_example_test.go`

```go
package main

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/require"
    "go.uber.org/zap"

    "github.com/BwCloudWeGo/bw-cli/pkg/config"
)

func TestRunMongoCollectionExampleUsesCurrentConfig(t *testing.T) {
    if os.Getenv("APP_RUN_NOTE_MONGODB_EXAMPLE") != "true" {
        t.Skip("set APP_RUN_NOTE_MONGODB_EXAMPLE=true to run this MongoDB example against configs/config.yaml")
    }

    previous := config.GlobalConfig
    defer func() { config.GlobalConfig = previous }()

    configPath := filepath.Join("..", "..", "configs", "config.yaml")
    require.NoError(t, config.InitGlobal(configPath))
    cfg := config.MustGlobal()

    document, err := runMongoCollectionExample(context.Background(), cfg, zap.NewNop())
    require.NoError(t, err)
    require.NotNil(t, document)
    require.Equal(t, cfg.App.NoteServiceName, document.Service)
}
```

运行：

```bash
export APP_RUN_NOTE_MONGODB_EXAMPLE=true
go test ./cmd/note -run TestRunMongoCollectionExampleUsesCurrentConfig -v
```

运行前确认 `configs/config.yaml` 中的 `mongodb.*` 已经指向可访问的 MongoDB。

## 24. 常见错误排查

### 24.1 server selection timeout

现象：

```text
server selection error: context deadline exceeded
```

排查：

- MongoDB 服务是否已经启动。
- `configs/config.yaml` 中的 `mongodb.uri` 地址和端口是否正确。
- 当前服务所在机器是否能访问该地址。
- 副本集名称是否正确

### 24.2 authentication failed

排查：

- 用户名是否正确。
- 密码是否正确。
- `uri` 中的认证库参数是否正确。
- URI 是否被 shell 特殊字符影响。复杂密码建议 URL encode。

### 24.3 no documents in result

`FindOne` 没查到数据时会返回：

```go
mongo.ErrNoDocuments
```

处理方式：

```go
if errors.Is(err, mongo.ErrNoDocuments) {
    return model.NoteDocument{}, apperrors.NewNotFound("note document not found")
}
```

### 24.4 cursor 泄漏

所有 `Find` 返回的 cursor 都要关闭：

```go
defer cursor.Close(ctx)
```

### 24.5 每次请求都创建 client

不要这样做。`mongo.Client` 内部维护连接池，一个进程创建一次即可。每个请求创建 client 会导致：

- 连接数暴涨。
- 请求延迟升高。
- MongoDB 端连接资源被耗尽。
- 服务关闭时资源难以释放。

### 24.6 查询慢

排查步骤：

1. 看查询条件。
2. 看排序字段。
3. 用 `explain("executionStats")` 看扫描情况。
4. 为查询条件和排序字段创建匹配索引。
5. 限制单次返回数量。

## 25. 课堂练习

### 练习 1：命令行 CRUD

目标：

- 用 `mongosh` 插入 3 条笔记。
- 查询某个作者的笔记。
- 修改其中一条笔记标题。
- 删除一条笔记。

### 练习 2：创建索引并分析查询

目标：

- 为 `note_id` 创建唯一索引。
- 为 `author_id + updated_at` 创建组合索引。
- 使用 `explain("executionStats")` 对比有索引和无索引时的查询结果。

### 练习 3：在 Go 中保存笔记文档

目标：

- 创建 `NoteDocument`。
- 创建 `NoteMongoRepository`。
- 实现 `Save` 和 `FindByNoteID`。
- 写一个集成测试验证保存和查询。

### 练习 4：实现分页查询

目标：

- 实现 `ListByAuthor`。
- 限制 `limit` 最大值为 100。
- 按 `updated_at` 倒序返回。
- 为查询创建合适索引。

## 26. 上线前检查清单

- MongoDB 配置文件按环境管理，生产配置不要混入本地默认配置。
- 生产密码只写入部署环境实际使用的安全配置文件。
- URI 中设置了正确的认证库参数。
- 副本集环境中设置了正确的 `replicaSet`。
- 关键查询都有索引。
- 大列表接口限制了 `limit`。
- 所有 cursor 都正确关闭。
- 一个进程只创建一个 MongoDB client。
- 服务退出时调用 `Disconnect`。
- 慢查询、连接数、错误率接入监控。
- 仓储层日志包含 collection、operation、latency、matched_count、modified_count、error。

## 27. 学习路线总结

推荐教学顺序：

1. 先用 `mongosh` 学会 database、collection、document。
2. 再用 `insertOne`、`find`、`updateOne`、`deleteOne` 理解 CRUD。
3. 再讲索引和 `explain`。
4. 然后接入 Go 项目，创建 MongoDB client。
5. 再按 DDD 分层写 `model` 接口和 `repo` 实现。
6. 最后讲分页、事务、测试和生产排错。

MongoDB 的关键不是“会连上”，而是知道什么数据适合放进文档、如何设计查询、如何配索引，以及如何把数据库细节限制在仓储层。
