# MongoDB 使用教程

这份文档说明如何在通过 `bw-cli` 生成的项目中使用 MongoDB。脚手架已经内置 `pkg/mongox` 和 `config.MongoDB`，你只需要配置连接信息，在业务服务启动时创建 MongoDB client，然后在 `repo` 层封装集合操作。

## 1. 适用场景

MongoDB 适合存储结构灵活、读写形态偏文档的数据，例如：

- 内容草稿、笔记正文快照、富文本 JSON。
- 用户扩展资料、三方平台原始回调。
- 操作审计日志、行为事件落库。
- 需要按业务字段组合查询，但不适合强关系建模的数据。

不建议把强事务、强外键约束、复杂多表聚合的核心账务数据直接迁到 MongoDB。关系明确的主业务表仍建议使用 MySQL 或 PostgreSQL。

## 2. 配置项

默认配置在 `configs/config.yaml`：

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
| `uri` | MongoDB 连接串，生产环境应包含账号、密码、节点、认证库和副本集参数。 |
| `database` | 默认业务数据库名。 |
| `app_name` | 写入 MongoDB client metadata，便于在监控和慢查询中识别服务。 |
| `min_pool_size` | 每个 MongoDB server 的最小连接池数量。 |
| `max_pool_size` | 每个 MongoDB server 的最大连接池数量。 |
| `connect_timeout_seconds` | 建立 TCP 连接的超时时间。 |
| `server_selection_timeout_seconds` | 选择可用 server 的超时时间。 |

生产环境建议使用环境变量，不要把密码写进 Git：

```bash
export APP_MONGODB_URI='mongodb://app:replace-with-real-password@mongo-1.example.com:27017,mongo-2.example.com:27017/app?replicaSet=rs0&authSource=admin'
export APP_MONGODB_DATABASE='app'
export APP_MONGODB_APP_NAME='note-service'
export APP_MONGODB_MIN_POOL_SIZE=2
export APP_MONGODB_MAX_POOL_SIZE=100
export APP_MONGODB_CONNECT_TIMEOUT_SECONDS=10
export APP_MONGODB_SERVER_SELECTION_TIMEOUT_SECONDS=5
```

## 3. 本地启动 MongoDB

脚手架的 `docker-compose.yml` 已包含 `mongodb` 服务。如果你使用 compose 里的服务，连接串需要带上认证信息：

```bash
docker compose up -d mongodb

export APP_MONGODB_URI='mongodb://bw:bw-secret@127.0.0.1:27017/xiaolanshu?authSource=admin'
export APP_MONGODB_DATABASE='xiaolanshu'
```

验证连接：

```bash
docker compose exec mongodb mongosh \
  'mongodb://bw:bw-secret@127.0.0.1:27017/xiaolanshu?authSource=admin' \
  --eval 'db.runCommand({ ping: 1 })'
```

如果你本机已经安装了无认证 MongoDB，可以直接使用默认 `mongodb://127.0.0.1:27017`。

## 4. 在服务启动时创建客户端

MongoDB client 是连接池对象，应该在服务启动时创建一次，进程退出时关闭。不要在每个 HTTP/gRPC 请求里创建 client。

示例放在某个服务的 `cmd/<service>/main.go`：

```go
package main

import (
    "context"
    "time"

    "go.uber.org/zap"
    "go.mongodb.org/mongo-driver/v2/mongo"

    "github.com/BwCloudWeGo/bw-cli/pkg/config"
    "github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

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

关闭客户端：

```go
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := mongoClient.Disconnect(ctx); err != nil {
        log.Error("disconnect mongodb failed", zap.Error(err))
    }
}()
```

## 5. 建议的目录位置

MongoDB 属于基础设施实现，应该放在业务服务的 `repo` 层：

```text
internal/note
  ├── model
  ├── service
  ├── repo
  │   ├── gorm_repository.go
  │   └── mongo_repository.go
  └── handler
```

`model` 只定义领域对象和仓储接口，不直接引用 MongoDB SDK。`repo/mongo_repository.go` 负责 BSON 结构、集合名、索引和查询。

## 6. 文档模型示例

示例：为笔记内容存一份 MongoDB 文档快照。

```go
package repo

import (
    "time"

    "go.mongodb.org/mongo-driver/v2/bson"
)

type NoteDocument struct {
    ID        bson.ObjectID `bson:"_id,omitempty"`
    NoteID    string        `bson:"note_id"`
    AuthorID  string        `bson:"author_id"`
    Title     string        `bson:"title"`
    Content   string        `bson:"content"`
    Status    string        `bson:"status"`
    CreatedAt time.Time     `bson:"created_at"`
    UpdatedAt time.Time     `bson:"updated_at"`
}
```

建议：

- `_id` 使用 `bson.ObjectID`，业务主键单独存 `note_id`。
- 时间字段统一使用 `time.Time`，由服务端写入。
- API 响应 DTO 不要直接复用 MongoDB 文档结构，避免 BSON tag 泄漏到接口层。

## 7. 仓储封装示例

```go
package repo

import (
    "context"
    "time"

    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
)

type NoteMongoRepository struct {
    collection *mongo.Collection
}

func NewNoteMongoRepository(client *mongo.Client, database string) *NoteMongoRepository {
    return &NoteMongoRepository{
        collection: client.Database(database).Collection("note_documents"),
    }
}

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
    })
    return err
}

func (r *NoteMongoRepository) Save(ctx context.Context, doc NoteDocument) error {
    now := time.Now()
    set := bson.M{
        "note_id":    doc.NoteID,
        "author_id":  doc.AuthorID,
        "title":      doc.Title,
        "content":    doc.Content,
        "status":     doc.Status,
        "updated_at": now,
    }

    _, err := r.collection.UpdateOne(
        ctx,
        bson.M{"note_id": doc.NoteID},
        bson.M{
            "$set": set,
            "$setOnInsert": bson.M{
                "_id":        bson.NewObjectID(),
                "created_at": now,
            },
        },
        options.UpdateOne().SetUpsert(true),
    )
    return err
}

func (r *NoteMongoRepository) FindByNoteID(ctx context.Context, noteID string) (NoteDocument, error) {
    var doc NoteDocument
    err := r.collection.FindOne(ctx, bson.M{"note_id": noteID}).Decode(&doc)
    return doc, err
}

func (r *NoteMongoRepository) ListByAuthor(ctx context.Context, authorID string, limit int64) ([]NoteDocument, error) {
    cursor, err := r.collection.Find(
        ctx,
        bson.M{"author_id": authorID},
        options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetLimit(limit),
    )
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var docs []NoteDocument
    if err := cursor.All(ctx, &docs); err != nil {
        return nil, err
    }
    return docs, nil
}
```

## 8. 在 service 层使用

`service` 层不要拼 BSON 查询。它只依赖你定义的仓储接口：

```go
package model

import "context"

type NoteDocumentRepository interface {
    Save(ctx context.Context, doc NoteDocument) error
    FindByNoteID(ctx context.Context, noteID string) (NoteDocument, error)
}
```

然后在 `repo` 层实现这个接口，在 `cmd/<service>/main.go` 里完成依赖注入。

## 9. 分页建议

小数据量可以用 `skip + limit`：

```go
options.Find().
    SetSort(bson.D{{Key: "updated_at", Value: -1}}).
    SetSkip(0).
    SetLimit(20)
```

大数据量建议使用游标分页，例如上一页最后一条记录的 `updated_at` 和 `_id`：

```go
filter := bson.M{
    "author_id": authorID,
    "$or": []bson.M{
        {"updated_at": bson.M{"$lt": lastUpdatedAt}},
        {"updated_at": lastUpdatedAt, "_id": bson.M{"$lt": lastID}},
    },
}
```

对应索引：

```go
bson.D{{Key: "author_id", Value: 1}, {Key: "updated_at", Value: -1}, {Key: "_id", Value: -1}}
```

## 10. 事务使用

MongoDB 事务需要副本集或分片集群。本地单节点如果没有以 replica set 模式启动，事务会失败。只在确实需要多集合原子写入时使用事务；单文档更新天然具备原子性。

事务伪代码：

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

## 11. 常见问题

### server selection timeout

含义是 driver 在 `server_selection_timeout_seconds` 时间内找不到可用节点。检查：

- `APP_MONGODB_URI` 中的 host 和端口是否可达。
- 副本集名称 `replicaSet` 是否正确。
- 容器内访问不要写 `127.0.0.1`，要写 compose 服务名，例如 `mongodb:27017`。

### authentication failed

检查：

- 用户名、密码是否正确。
- `authSource` 是否正确。使用 compose 内置 root 用户时通常是 `authSource=admin`。
- 数据库用户是否有目标 database 的读写权限。

### cursor 泄漏

所有 `Find` 返回的 cursor 都要关闭：

```go
defer cursor.Close(ctx)
```

### 每次请求都创建 client

不要这样做。MongoDB client 内部有连接池，每个进程创建一次即可。每次请求创建 client 会导致连接数飙升、延迟变高，也会让数据库端连接池不稳定。

## 12. 推荐上线检查清单

- `APP_MONGODB_URI` 来自密钥系统或环境变量，不写入 Git。
- URI 包含副本集、认证库和 TLS 参数。
- 关键查询都有索引，索引名固定。
- `max_pool_size` 与服务实例数、MongoDB 最大连接数匹配。
- 慢查询和连接池指标接入监控。
- 仓储层记录集合名、操作名、耗时、匹配行数和错误。
