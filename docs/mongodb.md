# MongoDB 从 0 到 1 教学教程

这份文档用于教学，不只是告诉你“配置什么参数”，而是按真实学习路径带你从零开始理解 MongoDB，并把它接入 `bw-cli` 生成的 Go 微服务项目。

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

关系型数据库常见结构：

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

### 3.2 检查 Docker

```bash
docker version
docker compose version
```

如果这两个命令不可用，先安装 Docker Desktop。

### 3.3 检查 bw-cli 项目

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

## 4. 本地启动 MongoDB

脚手架的 `docker-compose.yml` 已经内置了 MongoDB：

```yaml
mongodb:
  image: mongo:7
  environment:
    MONGO_INITDB_ROOT_USERNAME: bw
    MONGO_INITDB_ROOT_PASSWORD: bw-secret
    MONGO_INITDB_DATABASE: xiaolanshu
  volumes:
    - mongodb_data:/data/db
  ports:
    - "27017:27017"
```

启动 MongoDB：

```bash
docker compose up -d mongodb
```

查看容器：

```bash
docker compose ps mongodb
```

查看日志：

```bash
docker compose logs -f mongodb
```

连接串：

```text
mongodb://bw:bw-secret@127.0.0.1:27017/xiaolanshu?authSource=admin
```

说明：

- `bw`：用户名。
- `bw-secret`：密码。
- `127.0.0.1:27017`：本机访问 MongoDB。
- `xiaolanshu`：业务数据库。
- `authSource=admin`：使用 root 用户时，认证库是 `admin`。

## 5. 使用 mongosh 做第一次连接

进入容器内执行 `mongosh`：

```bash
docker compose exec mongodb mongosh \
  'mongodb://bw:bw-secret@127.0.0.1:27017/xiaolanshu?authSource=admin'
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
| `database` | 默认业务数据库 |
| `app_name` | 客户端名称，便于监控识别 |
| `min_pool_size` | 最小连接池数量 |
| `max_pool_size` | 最大连接池数量 |
| `connect_timeout_seconds` | TCP 建连超时 |
| `server_selection_timeout_seconds` | 选择可用节点超时 |

本地使用 compose 里的 MongoDB 时，建议用环境变量覆盖：

```bash
export APP_MONGODB_URI='mongodb://bw:bw-secret@127.0.0.1:27017/xiaolanshu?authSource=admin'
export APP_MONGODB_DATABASE='xiaolanshu'
export APP_MONGODB_APP_NAME='app-service'
```

生产环境示例：

```bash
export APP_MONGODB_URI='mongodb://app:replace-with-real-password@mongo-1.example.com:27017,mongo-2.example.com:27017/app?replicaSet=rs0&authSource=admin'
export APP_MONGODB_DATABASE='app'
export APP_MONGODB_APP_NAME='app-service'
export APP_MONGODB_MIN_POOL_SIZE=2
export APP_MONGODB_MAX_POOL_SIZE=100
export APP_MONGODB_CONNECT_TIMEOUT_SECONDS=10
export APP_MONGODB_SERVER_SELECTION_TIMEOUT_SECONDS=5
```

生产密码不要写进 `configs/config.yaml`，应该通过环境变量、Kubernetes Secret 或配置中心注入。

## 9. 脚手架内置的 mongox 包

脚手架提供了 `pkg/mongox`：

```text
pkg/mongox
  ├── mongox.go
  └── mongox_test.go
```

它做三件事：

- 定义 MongoDB 连接配置。
- 创建官方 driver 的 `mongo.Client`。
- 提供 `Ping` 方法验证连接。

核心用法：

```go
client, err := mongox.NewClient(mongox.Config{
    URI:                    cfg.MongoDB.URI,
    Database:               cfg.MongoDB.Database,
    AppName:                cfg.MongoDB.AppName,
    MinPoolSize:            cfg.MongoDB.MinPoolSize,
    MaxPoolSize:            cfg.MongoDB.MaxPoolSize,
    ConnectTimeout:         cfg.MongoDB.ConnectTimeout(),
    ServerSelectionTimeout: cfg.MongoDB.ServerSelectionTimeout(),
})
```

注意：`mongo.Client` 是连接池对象，一个进程创建一次即可。不要每个请求创建一次。

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
}

func NewNoteMongoRepository(client *mongo.Client, database string) *NoteMongoRepository {
    return &NoteMongoRepository{
        collection: client.Database(database).Collection("note_documents"),
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

    _, err := r.collection.UpdateOne(
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
    var record noteDocumentRecord
    err := r.collection.FindOne(ctx, bson.M{"note_id": noteID}).Decode(&record)
    if err != nil {
        return model.NoteDocument{}, err
    }
    return toNoteDocument(record), nil
}
```

如果没查到，driver 返回 `mongo.ErrNoDocuments`。业务层可以把它转换成自己的业务错误：

```go
if errors.Is(err, mongo.ErrNoDocuments) {
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

    cursor, err := r.collection.Find(
        ctx,
        bson.M{"author_id": authorID},
        options.Find().
            SetSort(bson.D{{Key: "updated_at", Value: -1}}).
            SetLimit(limit),
    )
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var records []noteDocumentRecord
    if err := cursor.All(ctx, &records); err != nil {
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
- `Find` 返回 cursor，必须 `defer cursor.Close(ctx)`。
- 查询字段和排序字段要匹配索引。

## 19. 删除文档

```go
func (r *NoteMongoRepository) DeleteByNoteID(ctx context.Context, noteID string) error {
    _, err := r.collection.DeleteOne(ctx, bson.M{"note_id": noteID})
    return err
}
```

如果业务需要软删除，不要直接删除，可以更新状态：

```go
_, err := r.collection.UpdateOne(
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

MongoDB 集成测试依赖本地服务，不应该默认强制运行。推荐用环境变量控制。

文件：`internal/note/repo/mongo_repository_test.go`

```go
package repo_test

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/stretchr/testify/require"

    "github.com/BwCloudWeGo/bw-cli/internal/note/model"
    "github.com/BwCloudWeGo/bw-cli/internal/note/repo"
    "github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

func TestNoteMongoRepositorySaveAndFind(t *testing.T) {
    uri := os.Getenv("APP_MONGODB_URI")
    if uri == "" {
        t.Skip("APP_MONGODB_URI is required for mongodb integration test")
    }
    database := os.Getenv("APP_MONGODB_DATABASE")
    if database == "" {
        database = "xiaolanshu_test"
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    client, err := mongox.NewClient(mongox.Config{URI: uri, Database: database})
    require.NoError(t, err)
    defer client.Disconnect(ctx)

    repository := repo.NewNoteMongoRepository(client, database)
    require.NoError(t, repository.EnsureIndexes(ctx))

    doc := model.NoteDocument{
        NoteID:   "note-test-1",
        AuthorID: "user-test-1",
        Title:    "MongoDB integration test",
        Content:  "hello mongodb",
        Status:   "draft",
        Tags:     []string{"test"},
    }

    require.NoError(t, repository.Save(ctx, doc))

    got, err := repository.FindByNoteID(ctx, doc.NoteID)
    require.NoError(t, err)
    require.Equal(t, doc.NoteID, got.NoteID)
    require.Equal(t, doc.Title, got.Title)
}
```

运行：

```bash
docker compose up -d mongodb

export APP_MONGODB_URI='mongodb://bw:bw-secret@127.0.0.1:27017/xiaolanshu_test?authSource=admin'
export APP_MONGODB_DATABASE='xiaolanshu_test'

go test ./internal/note/repo -run TestNoteMongoRepositorySaveAndFind -v
```

## 24. 常见错误排查

### 24.1 server selection timeout

现象：

```text
server selection error: context deadline exceeded
```

排查：

- MongoDB 是否启动：`docker compose ps mongodb`
- 端口是否正确：`27017`
- 在宿主机访问用 `127.0.0.1:27017`
- 在 compose 容器内访问用 `mongodb:27017`
- 副本集名称是否正确

### 24.2 authentication failed

排查：

- 用户名是否正确。
- 密码是否正确。
- `authSource` 是否正确。使用本教程 compose 里的 root 用户时，应该是 `authSource=admin`。
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

- MongoDB URI 不写入 Git。
- 生产密码来自环境变量、密钥系统或配置中心。
- URI 中设置了正确的 `authSource`。
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
