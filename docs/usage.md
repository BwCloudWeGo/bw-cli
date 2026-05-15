# 使用说明

这份文档按真实使用流程写：先准备脚手架仓库，再安装 `bw-cli`，然后生成干净项目、初始化依赖、启动服务、修改配置、扩展业务。需要 user/note 示例时，使用单独的 `bw-cli demo` 命令。

## 1. 准备环境

### 1.1 检查 Go

```bash
go version
```

要求：

```text
go1.25+
```

### 1.2 检查 protoc

```bash
protoc --version
```

如果没有安装，需要先安装 Protocol Buffers 编译器。

### 1.3 安装 Go 的 proto 插件

在脚手架仓库根目录执行：

```bash
make tools
```

它会安装：

```text
protoc-gen-go
protoc-gen-go-grpc
```

### 1.4 验证脚手架自身

```bash
make proto
make test
```

Makefile 只调用 Go 命令，不依赖 `find`、`sed`、`if [ ... ]` 等 Unix shell 语法。`make proto` 内部会调用 `tools/protogen`，由 Go 自动适配 Windows、macOS、Linux 的路径分隔符和 proto 插件目录。
Windows 仍需要安装 GNU Make；如果没有 `make`，可以直接执行等价的 Go 命令，例如 `go run ./tools/protogen`、`go test ./...`、`go run ./cmd/gateway`。

预期结果：

```text
所有 package 测试通过，无 FAIL
```

## 2. 作为脚手架维护者：发布 bw-cli

如果你要把这个脚手架放到 Git 仓库里给团队使用，先做这几步。

### 2.1 确认 go.mod module

打开 `go.mod`，确认 module 是真实仓库地址。

示例：

```go
module github.com/BwCloudWeGo/bw-cli
```

不要使用本地临时 module 名称，否则远程安装会失败。

### 2.2 提交到 Git 仓库

```bash
git init
git add .
git commit -m "init bw-cli scaffold"
git remote add origin https://github.com/BwCloudWeGo/bw-cli.git
git push -u origin main
```

`bw-cli new` 默认使用官方仓库的 `main` 分支；企业内部 fork 时仍可以通过 `--repo` 和 `--branch` 覆盖。

### 2.3 验证远程安装命令

在任意目录执行：

```bash
go install github.com/BwCloudWeGo/bw-cli/cmd/bw-cli@latest
```

确认命令已安装：

```bash
bw-cli new -h
```

能看到 `new` 命令参数说明即可。

## 3. 作为脚手架使用者：安装 bw-cli

推荐方式是 `go install`。

```bash
go install github.com/BwCloudWeGo/bw-cli/cmd/bw-cli@latest
```

确认安装：

```bash
bw-cli new -h
```

如果是引用公共基础包，使用 `go get` 添加依赖：

```bash
go get github.com/BwCloudWeGo/bw-cli/pkg/logger
go get github.com/BwCloudWeGo/bw-cli/pkg/mysqlx
go get github.com/BwCloudWeGo/bw-cli/pkg/postgresx
go get github.com/BwCloudWeGo/bw-cli/pkg/mongox
go get github.com/BwCloudWeGo/bw-cli/pkg/filex
```

Go 1.17+ 推荐 `go install ...@latest` 安装命令行工具，`go get` 用来添加库依赖。

## 4. 生成新项目

`bw-cli` 支持两种模板模式：

- `bw-cli new`：生成干净框架，不带 user/note demo。
- `bw-cli demo`：生成演示项目，保留 user/note demo，适合教学和试跑。

日常业务项目推荐使用 `new`。

### 4.1 生成干净业务项目

这是团队正式使用时最常见的方式。

```bash
bw-cli new my-service \
  --module github.com/acme/my-service \
  --tidy
```

参数解释：

- `new my-service`：生成目录是当前目录下的 `my-service`。
- `--module github.com/acme/my-service`：新项目的 Go module。
- `--tidy`：生成完成后自动执行 `go mod tidy`。

不传 `--repo` 和 `--branch` 时，默认使用：

```text
repo   https://github.com/BwCloudWeGo/bw-cli.git
branch main
```

生成后会得到：

```text
my-service
  ├── api
  ├── cmd
  ├── configs
  ├── internal
  ├── pkg
  ├── docs
  ├── Makefile
  ├── go.mod
  └── README.md
```

干净项目不会包含：

```text
cmd/bw-cli
cmd/user
cmd/note
internal/user
internal/note
api/proto/user
api/proto/note
api/gen/user
api/gen/note
```

### 4.2 生成演示项目

如果要学习完整调用链，使用 `demo` 命令：

```bash
bw-cli demo demo-service \
  --module github.com/acme/demo-service \
  --tidy
```

演示项目会保留：

```text
user-service
note-service
gateway -> user/note gRPC client
/api/v1/users
/api/v1/notes
```

### 4.3 从本地脚手架目录生成

适合你正在开发或调试脚手架时使用。

```bash
git clone https://github.com/BwCloudWeGo/bw-cli.git
cd bw-cli
go install ./cmd/bw-cli

bw-cli new ../my-service \
  --module github.com/acme/my-service \
  --source . \
  --tidy
```

参数解释：

- `../my-service`：新项目生成到脚手架目录的同级目录。
- `--source .`：用当前目录作为脚手架模板来源。
- `--module github.com/acme/my-service`：新项目 module。

从本地源码生成演示项目：

```bash
bw-cli demo ../demo-service \
  --module github.com/acme/demo-service \
  --source . \
  --tidy
```

### 4.4 bw-cli 生成时做了什么

生成过程包含：

1. 默认从官方仓库克隆脚手架；如果传了 `--source`，则复制本地脚手架。
2. 跳过 `.git`、`.idea`、`logs`、`data`、`tmp` 等运行时目录。
3. 替换 `go.mod` 和源码中的 module 路径。
4. 跳过已生成的 `*.pb.go`，避免破坏 protobuf 原始描述符。
5. 替换 `.proto` 文件中的 `go_package`，后续可通过 `make proto` 重新生成代码。
6. 移除脚手架自身 CLI 代码，生成项目不会继续携带 `cmd/bw-cli` 和 `pkg/scaffold`。
7. `new` 会移除 user/note 示例业务；`demo` 会保留示例业务。
8. 重写生成项目内的 README、usage、architecture、toolkit、mongodb 文档，避免出现已被移除的 CLI 或示例目录说明。
9. 如果指定 `--tidy`，自动执行 `go mod tidy`。

## 5. 初始化生成后的项目

进入新项目目录：

```bash
cd my-service
```

安装 proto 插件：

```bash
make tools
```

重新生成 proto 代码：

```bash
make proto
```

整理依赖：

```bash
go mod tidy
```

运行测试：

```bash
make test
```

预期结果：

```text
所有 package 测试通过
```

## 6. 配置项目

主配置文件：

```text
configs/config.yaml
```

### 6.1 服务名配置

```yaml
app:
  name: app
  env: local
  gateway_service_name: gateway
```

这些值会用于日志文件名、日志字段和可观测性服务名。

### 6.2 HTTP 配置

```yaml
http:
  host: 0.0.0.0
  port: 8080
  read_timeout_seconds: 5
  write_timeout_seconds: 10
```

修改端口后启动 gateway：

```bash
APP_HTTP_PORT=8081 make run-gateway
```

Windows PowerShell 使用：

```powershell
$env:APP_HTTP_PORT="8081"; make run-gateway
```

### 6.3 gRPC 配置

```yaml
grpc:
  host: 0.0.0.0
```

干净项目默认不创建具体 gRPC 服务端口。执行 `bw-cli service <name>` 后，服务默认端口写在 `cmd/<name>/main.go`，可用 `APP_<SERVICE>_GRPC_PORT` 覆盖；gateway 目标地址可用 `APP_<SERVICE>_GRPC_TARGET` 覆盖，不需要先改 `configs/config.yaml`。

### 6.4 SQLite 默认配置

默认使用 SQLite，适合本地快速运行：

```yaml
database:
  driver: sqlite
  dsn: data/xiaolanshu.db
```

直接启动服务即可自动创建本地数据库文件。

### 6.5 切换到 MySQL

配置文件中不要写假账号密码。推荐通过环境变量注入：

```bash
export APP_DATABASE_DRIVER=mysql
export APP_MYSQL_DSN='user:pass@tcp(mysql.example.com:3306)/app?charset=utf8mb4&parseTime=True&loc=Local'
export APP_MYSQL_MAX_IDLE_CONNS=10
export APP_MYSQL_MAX_OPEN_CONNS=100
export APP_MYSQL_CONN_MAX_LIFETIME_SECONDS=3600
```

然后启动服务：

```bash
make run-gateway
```

注意：切换 MySQL 时，`database.driver` 决定使用哪个驱动，`mysql.*` 决定 MySQL 连接和连接池参数。

### 6.6 切换到 PostgreSQL

PostgreSQL 也走 Gorm 入口，适合替代 MySQL 作为主业务库。配置文件中不要写生产账号密码，推荐通过环境变量注入：

```bash
export APP_DATABASE_DRIVER=postgres
export APP_POSTGRESQL_DSN='host=postgres.example.com user=app password=replace-with-real-password dbname=app port=5432 sslmode=require TimeZone=Asia/Shanghai'
export APP_POSTGRESQL_MAX_IDLE_CONNS=10
export APP_POSTGRESQL_MAX_OPEN_CONNS=100
export APP_POSTGRESQL_CONN_MAX_LIFETIME_SECONDS=3600
```

然后启动服务：

```bash
make run-gateway
```

支持的关系型驱动值：

```text
sqlite
mysql
postgres
postgresql
pg
```

注意：切换 PostgreSQL 时，`database.driver` 决定使用哪个驱动，`postgresql.*` 决定 PostgreSQL 连接和连接池参数。

### 6.7 MongoDB 配置

MongoDB 不走 Gorm，适合存储内容快照、扩展属性、操作日志、草稿、搜索前的宽表文档等。默认配置：

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

服务启动时只读取 `configs/config.yaml` 中的 `mongodb.*` 配置。需要账号密码时，填写 `username`、`password`；需要连接副本集或指定认证库时，把完整连接串写入 `uri`。

脚手架调用全流程见 [MongoDB 调用示例全流程](mongo-call-examples.md)。

业务仓储不要直接散落调用 MongoDB driver。推荐统一使用 `pkg/mongox` 提供的公共操作类：

```go
type NoteDocument struct {
    ID    string `bson:"_id"`
    Title string `bson:"title"`
}

func (NoteDocument) MongoCollectionName() string {
    return "notes"
}

db := mongox.Database(client, cfg.MongoDB.Database)
notes := mongox.NewDocumentStore[NoteDocument](db)

_, err := notes.UpsertByID(ctx, "note-1", &NoteDocument{ID: "note-1", Title: "MongoDB note"})
note, err := notes.FindByID(ctx, "note-1")
```

### 6.8 文件上传配置

脚手架通过 `pkg/filex` 封装文件上传能力。业务代码只依赖统一的 `filex.Uploader` 接口，实际存储可以通过配置切换为 MinIO、阿里云 OSS、七牛云 Kodo 或腾讯云 COS。

默认配置：

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

支持的存储方式：

```text
minio
oss
qiniu
cos
```

MinIO 环境变量示例：

```bash
export APP_FILE_STORAGE_PROVIDER=minio
export APP_FILE_STORAGE_MAX_SIZE_MB=100
export APP_FILE_STORAGE_OBJECT_PREFIX=uploads
export APP_FILE_STORAGE_PUBLIC_BASE_URL='https://cdn.example.com'
export APP_FILE_STORAGE_MINIO_ENDPOINT='127.0.0.1:9000'
export APP_FILE_STORAGE_MINIO_ACCESS_KEY_ID='replace-with-real-access-key'
export APP_FILE_STORAGE_MINIO_SECRET_ACCESS_KEY='replace-with-real-secret-key'
export APP_FILE_STORAGE_MINIO_BUCKET='app-files'
export APP_FILE_STORAGE_MINIO_USE_SSL=false
```

阿里云 OSS：

```bash
export APP_FILE_STORAGE_PROVIDER=oss
export APP_FILE_STORAGE_OSS_ENDPOINT='https://oss-cn-hangzhou.aliyuncs.com'
export APP_FILE_STORAGE_OSS_ACCESS_KEY_ID='replace-with-real-access-key'
export APP_FILE_STORAGE_OSS_ACCESS_KEY_SECRET='replace-with-real-secret-key'
export APP_FILE_STORAGE_OSS_BUCKET='app-files'
```

七牛云 Kodo：

```bash
export APP_FILE_STORAGE_PROVIDER=qiniu
export APP_FILE_STORAGE_QINIU_ACCESS_KEY='replace-with-real-access-key'
export APP_FILE_STORAGE_QINIU_SECRET_KEY='replace-with-real-secret-key'
export APP_FILE_STORAGE_QINIU_BUCKET='app-files'
export APP_FILE_STORAGE_QINIU_REGION='z0'
export APP_FILE_STORAGE_QINIU_USE_HTTPS=true
```

腾讯云 COS：

```bash
export APP_FILE_STORAGE_PROVIDER=cos
export APP_FILE_STORAGE_COS_SECRET_ID='replace-with-real-secret-id'
export APP_FILE_STORAGE_COS_SECRET_KEY='replace-with-real-secret-key'
export APP_FILE_STORAGE_COS_BUCKET='app-files-1250000000'
export APP_FILE_STORAGE_COS_REGION='ap-guangzhou'
```

业务调用示例：

```go
uploader, err := filex.NewUploader(cfg.FileStorage)
if err != nil {
    return err
}

result, err := uploader.Upload(ctx, filex.UploadRequest{
    Reader:      file,
    Filename:    header.Filename,
    ContentType: header.Header.Get("Content-Type"),
    Size:        header.Size,
})
```

完整上传接口、返回结构和 Gin handler 示例见 [工具组件总览与调用流程](toolkit.md)。

### 6.9 Redis 配置

```yaml
redis:
  addr: 127.0.0.1:6379
  username: ""
  password: ""
  db: 0
  pool_size: 10
```

生产环境建议通过环境变量注入密码：

```bash
export APP_REDIS_ADDR='redis.example.com:6379'
export APP_REDIS_PASSWORD='replace-with-real-password'
```

### 6.10 Elasticsearch 配置

```yaml
elasticsearch:
  addresses:
    - http://127.0.0.1:9200
  username: ""
  password: ""
```

生产环境示例：

```bash
export APP_ELASTICSEARCH_ADDRESSES='https://es.example.com:9200'
export APP_ELASTICSEARCH_USERNAME='elastic'
export APP_ELASTICSEARCH_PASSWORD='replace-with-real-password'
```

### 6.11 Kafka 配置

```yaml
kafka:
  brokers:
    - 127.0.0.1:9092
  topic: xiaolanshu-events
  group_id: xiaolanshu-consumer
```

生产环境示例：

```bash
export APP_KAFKA_BROKERS='kafka-1.example.com:9092,kafka-2.example.com:9092'
export APP_KAFKA_TOPIC='business-events'
export APP_KAFKA_GROUP_ID='business-consumer'
```

### 6.12 CORS 配置

```yaml
middleware:
  cors:
    allow_origins:
      - "*"
    allow_methods:
      - GET
      - POST
      - PUT
      - PATCH
      - DELETE
      - OPTIONS
    allow_headers:
      - Origin
      - Content-Type
      - Authorization
      - X-Request-ID
    allow_credentials: false
```

生产环境建议把 `allow_origins` 改成明确域名：

```yaml
allow_origins:
  - https://console.example.com
```

### 6.13 JWT 配置

JWT 密钥默认不提供假值，必须配置：

```bash
export APP_MIDDLEWARE_JWT_SECRET='replace-with-a-real-secret'
export APP_MIDDLEWARE_JWT_ISSUER='xiaolanshu'
export APP_MIDDLEWARE_JWT_EXPIRE_SECONDS=7200
```

生成 token 示例：

```go
cfg := middleware.DefaultJWTConfig()
cfg.Secret = os.Getenv("APP_MIDDLEWARE_JWT_SECRET")

token, err := middleware.GenerateToken(cfg, middleware.JWTClaims{
    UserID: "user-1",
    Role:   "admin",
})
```

## 7. 启动服务

如果是 `bw-cli new` 生成的干净项目，默认只有 gateway 入口：

```bash
make run-gateway
```

端口监听成功后，控制台会输出：

```text
[Gateway Started]
  service: gateway
  env: local
  listen: 0.0.0.0:8080
  http: http://127.0.0.1:8080
  health: http://127.0.0.1:8080/healthz
  api: http://127.0.0.1:8080/api/v1
```

如果端口被占用，控制台会输出 `[Gateway Start Failed]`，并显示失败的监听地址和系统错误。

监听：

```text
0.0.0.0:8080
```

如果是 `bw-cli demo` 生成的演示项目，建议开三个终端。

### 7.1 演示项目：启动 user-service

```bash
make run-user
```

监听：

```text
0.0.0.0:9001
```

### 7.2 演示项目：启动 note-service

```bash
make run-note
```

监听：

```text
0.0.0.0:9002
```

### 7.3 启动 gateway

```bash
make run-gateway
```

监听：

```text
0.0.0.0:8080
```

### 7.4 查看日志

日志按服务名和日期生成：

```bash
ls logs
```

示例：

```text
gateway-2026-04-28.log
```

查看 gateway 日志：

```bash
tail -f logs/gateway-$(date +%F).log
```

## 8. 调用接口验证

### 8.1 干净项目：健康检查

```bash
curl -i http://localhost:8080/healthz
```

预期：

```text
HTTP/1.1 200 OK
{"status":"ok"}
```

### 8.2 演示项目：注册用户

以下接口只存在于 `bw-cli demo` 生成的演示项目，`bw-cli new` 生成的干净项目不会携带这些业务 demo。

```bash
curl -i -X POST http://localhost:8080/api/v1/users/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","display_name":"Ada","password":"secret123"}'
```

预期：

```json
{
  "request_id": "...",
  "data": {
    "id": "...",
    "email": "ada@example.com",
    "display_name": "Ada"
  }
}
```

记录返回的 `data.id`，后面创建笔记会用到。

### 8.3 登录用户

```bash
curl -i -X POST http://localhost:8080/api/v1/users/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"secret123"}'
```

预期返回用户信息。

### 8.4 查询用户

把 `<user_id>` 换成注册接口返回的用户 ID：

```bash
curl -i http://localhost:8080/api/v1/users/<user_id>
```

### 8.5 创建笔记

```bash
curl -i -X POST http://localhost:8080/api/v1/notes \
  -H 'Content-Type: application/json' \
  -d '{"author_id":"<user_id>","title":"DDD scaffold","content":"Gin plus gRPC demo"}'
```

预期：

```json
{
  "request_id": "...",
  "data": {
    "id": "...",
    "author_id": "<user_id>",
    "title": "DDD scaffold",
    "content": "Gin plus gRPC demo",
    "status": "DRAFT"
  }
}
```

记录返回的 `data.id`，发布笔记会用到。

### 8.6 发布笔记

把 `<note_id>` 换成创建笔记返回的笔记 ID：

```bash
curl -i -X POST http://localhost:8080/api/v1/notes/<note_id>/publish
```

预期 `status` 变为：

```text
PUBLISHED
```

## 9. 代码结构怎么读

### 9.1 HTTP 入参在哪里

干净项目默认不创建 HTTP 入参 DTO。新增业务时放在：

```text
internal/gateway/request
```

演示项目示例：

```text
RegisterUserRequest
LoginUserRequest
CreateNoteRequest
```

控制器不再定义请求结构体，只负责绑定请求和调用下游。

### 9.2 路由在哪里

```text
internal/gateway/router
```

路由按层级拆分：

```text
router.go       # Gin engine + 全局中间件
health.go       # /healthz
v1.go           # /api/v1
```

干净项目只保留 `router.go`、`health.go`、`v1.go`。新增业务时按业务单独创建路由文件，例如 `product_routes.go`、`order_routes.go`。

### 9.3 业务服务怎么看

以新增 `comment` 服务为例：

```text
internal/comment/model      # Comment 实体、错误、Repository 接口
internal/comment/dto
  ├── command.go            # CreateCommand、UpdateCommand、ListCommand
  └── comment.go            # CommentDTO、ListCommentDTO、FromComment
internal/comment/service
  └── service.go            # Create/Get/List 等业务用例
internal/comment/repo       # Gorm 或 MongoDB 实现
internal/comment/handler    # gRPC server
```

## 10. 新增一个业务服务

假设新增 `comment-service`，进入项目根目录执行：

```bash
bw-cli service comment --tidy
```

命令会自动读取当前项目的 `go.mod`，生成完整 CRUD 服务并执行 `make proto`。不传 `--port` 时默认端口是 `9100`，需要指定端口时使用：

```bash
bw-cli service comment --port 9103 --tidy
```

生成后不需要修改 `configs/config.yaml`：

- 服务名、默认 gRPC 端口和端口环境变量会写入 `cmd/comment/main.go`。
- 数据库继续读取项目已有的 `database`、`mysql`、`postgresql` 配置。
- SQLite 默认配置可直接本地运行，服务启动时会自动执行 `AutoMigrate`。
- `--tidy` 会在生成后执行 `go mod tidy`。

生成文件：

```text
api/proto/comment/v1/comment.proto
api/gen/comment/v1
cmd/comment/main.go
internal/comment/model
internal/comment/dto/command.go
internal/comment/dto/comment.go
internal/comment/service/service.go
internal/comment/repo/gorm_repository.go
internal/comment/repo/mongo_repository.go
internal/comment/handler
docs/services/comment.md
```

同时会在 `Makefile` 追加：

```bash
make run-comment
```

默认端口可通过环境变量覆盖：

```bash
export APP_COMMENT_GRPC_PORT=9104
make run-comment
```

Windows PowerShell：

```powershell
$env:APP_COMMENT_GRPC_PORT="9104"; make run-comment
```

### 10.1 生成后已经具备什么

`bw-cli service` 生成的是一条可编译、可启动、可继续扩展的基础 CRUD 调用链。用户可以直接基于生成代码开发业务，不需要先补空文件。

默认已经包含：

- `CreateComment`
- `GetComment`
- `ListComments`
- `UpdateComment`
- `DeleteComment`

如果项目包含 Gin gateway，命令还会同步生成 HTTP CRUD 入口：

```text
POST   /api/v1/comments
GET    /api/v1/comments
GET    /api/v1/comments/:id
PUT    /api/v1/comments/:id
DELETE /api/v1/comments/:id
```

默认调用链如下：

```text
HTTP client
  -> internal/gateway/router
  -> internal/gateway/handler
  -> gRPC client
  -> api/proto/comment/v1/comment.proto
  -> internal/comment/handler
  -> internal/comment/service
  -> internal/comment/model.Repository
  -> internal/comment/repo(Gorm)
  -> database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
```

默认启动使用 `repo/gorm_repository.go`。脚手架同时生成 `repo/mongo_repository.go`，里面已经通过 `MongoCollectionName()` 和 `mongox.NewDocumentStore[T]` 接好基础 CRUD。业务需要 MongoDB 时，在 `cmd/comment/main.go` 中用配置文件创建 Mongo client 和 database，然后把 repository 注入改为 `repo.NewMongoRepository(mongoDB, log)`。

服务端口不需要写进配置文件。`cmd/comment/main.go` 内置默认端口 `9103`，也支持环境变量覆盖：

```text
APP_COMMENT_GRPC_PORT
```

gateway 调用服务的目标地址也不需要写进配置文件，默认是 `127.0.0.1:9103`，需要覆盖时设置：

```bash
export APP_COMMENT_GRPC_TARGET=127.0.0.1:9103
```

### 10.2 每一层怎么写，为什么这么写

| 层级 | 放什么 | 怎么写 | 为什么这么写 |
| --- | --- | --- | --- |
| `api/proto/<service>/v1` | gRPC 协议 | 默认带 CRUD RPC，请按业务改 Request/Response 字段 | 先稳定外部契约，避免 handler/service 随意暴露内部模型 |
| `api/gen/<service>/v1` | 生成代码 | 只通过 `make proto` 生成，不手写 | 保持 proto 和 Go 类型一致，减少人为错误 |
| `cmd/<service>` | 服务启动入口 | 加载配置、初始化日志、打开数据库、注册 gRPC server | 入口负责组装依赖，业务逻辑不放在 main 中 |
| `internal/<service>/model` | 领域模型 | 写实体、业务错误、Repository 接口 | model 是业务核心，不依赖 Gin、gRPC、Gorm，方便测试和替换基础设施 |
| `internal/<service>/dto/command.go` | 业务用例入参 | 定义 `CreateCommand`、`UpdateCommand`、`ListCommand` 等命令对象 | handler 只组装命令，不堆业务字段 |
| `internal/<service>/dto/<service>.go` | 业务用例出参 | 定义 DTO，并把领域模型转成返回结构 | 对外不暴露领域模型和数据库模型 |
| `internal/<service>/service/service.go` | 业务流程编排 | 接收命令对象，调用领域模型和仓储接口 | service 表达业务流程，避免 handler 直接写业务 |
| `internal/<service>/repo` | 数据库实现 | 默认用 Gorm 实现 `model.Repository` | 数据库操作集中在 repo，业务层只面向接口 |
| `internal/<service>/handler` | gRPC 入站适配 | 把 proto request 转成 service command，把 DTO 转成 proto response | handler 只做协议转换，不写数据库和复杂业务 |
| `internal/gateway/request` | HTTP 入参 DTO | 定义 Gin bind/validate 结构体 | 控制器不堆请求字段，入参可复用和测试 |
| `internal/gateway/handler` | HTTP 控制器 | 参数绑定、调用下游 gRPC client、统一响应 | HTTP 层只处理 Web 协议，不直接操作数据库 |
| `internal/gateway/router` | 路由注册 | 按 `/api/v1/<business>` 分文件注册 | 路由按版本/业务拆分，避免所有接口堆在一个文件 |

### 10.3 model 层：写业务核心

生成后的 `model` 默认包含 `ID`、`Name`、`Description`、`CreatedAt`、`UpdatedAt`，这是 CRUD 示例字段。真实开发时，把 `Name/Description` 换成业务字段，把校验放在构造函数或实体方法里。

Repository 接口也放在 `model`：

```go
type Repository interface {
    Save(ctx context.Context, item *Comment) error
    FindByID(ctx context.Context, id string) (*Comment, error)
    List(ctx context.Context, offset int, limit int) ([]*Comment, int64, error)
    Delete(ctx context.Context, id string) error
}
```

这样做的原因是：业务层只关心“保存、查询、分页、删除”这些能力，不关心底层是 MySQL、PostgreSQL、MongoDB 还是测试 fake。

### 10.4 service 层：写业务流程

`dto` 与 `service` 已经按职责拆开，`service` 层只保留 `Create/Get/List/Update/Delete` 用例流程：

```text
internal/comment/dto/command.go      # 业务入参命令
internal/comment/dto/comment.go      # 业务出参 DTO 和 FromComment
internal/comment/service/service.go  # 业务流程编排
```

新增业务规则时写在 `service.go` 或 `model`，不要写在 handler。新增入参先放 `dto/command.go`，新增出参和转换放 `dto/comment.go`。

```go
func (s *Service) Create(ctx context.Context, cmd dto.CreateCommand) (*dto.CommentDTO, error) {
    item, err := model.NewComment(cmd.Name, cmd.Description)
    if err != nil {
        return nil, err
    }
    if err := s.repo.Save(ctx, item); err != nil {
        return nil, err
    }
    return dto.FromComment(item), nil
}
```

这样写的原因是：业务规则集中在 service/model，handler 变薄，repo 可替换，单元测试可以用 fake repository。生成的 `internal/comment/service/service_test.go` 已经给出 fake repository 的 CRUD 测试示例。

### 10.5 repo 层：数据库在哪里操作，如何操作

数据库操作只放在 `internal/<service>/repo`。默认使用 Gorm，`cmd/<service>/main.go` 负责打开数据库并注入 repo：

```go
db, err := database.Open(cfg.Database, cfg.MySQL, cfg.PostgreSQL, log)
repo := commentrepo.NewGormRepository(db, log)
svc := commentservice.NewService(repo, log)
```

生成后的 Gorm repo 已经实现：

- `AutoMigrate(db)`：创建或更新表结构。
- `Save(ctx, item)`：新增或更新。
- `FindByID(ctx, id)`：按 ID 查询，查不到返回 `model.ErrCommentNotFound`。
- `List(ctx, offset, limit)`：分页查询并返回总数。
- `Delete(ctx, id)`：按 ID 删除，删不到返回 `model.ErrCommentNotFound`。

数据库操作规则：

- `handler` 不直接操作数据库。
- `service` 不直接使用 `*gorm.DB`。
- `model` 不引入 Gorm tag，避免领域模型和数据库实现耦合。
- 查询、分页、事务、锁、索引相关实现都放在 `repo`。
- 需要事务时，在 repo 层内部使用 `db.Transaction(func(tx *gorm.DB) error { ... })`。
- 多数据源时保持接口不变，例如 `GormRepository`、`MongoRepository` 都实现 `model.Repository`。

### 10.6 handler 层：协议转换

`handler` 层已经生成 CRUD 方法。它只处理协议转换：

1. 从 proto request 取字段。
2. 组装 service command。
3. 调用 service。
4. 把 DTO 转成 proto response。
5. 把业务错误转成统一错误。

不要在 handler 中写 SQL、Gorm 查询、复杂业务判断。

### 10.7 gateway 层：HTTP 调用已经接好

`bw-cli service comment` 会自动生成：

```text
internal/gateway/handler/comment_handler.go
internal/gateway/request/comment_request.go
internal/gateway/router/comment_routes.go
```

并把 `internal/gateway/router/v1.go` 中的 `/api/v1` 路由自动挂上。用户启动 `comment-service` 和 `gateway` 后，可以直接通过 HTTP 调用基础 CRUD。

创建：

```bash
curl -i -X POST http://localhost:8080/api/v1/comments \
  -H 'Content-Type: application/json' \
  -d '{"name":"first","description":"created from gateway"}'
```

列表：

```bash
curl -i 'http://localhost:8080/api/v1/comments?page=1&page_size=20'
```

详情、更新、删除：

```bash
curl -i http://localhost:8080/api/v1/comments/<id>
curl -i -X PUT http://localhost:8080/api/v1/comments/<id> \
  -H 'Content-Type: application/json' \
  -d '{"name":"updated","description":"updated from gateway"}'
curl -i -X DELETE http://localhost:8080/api/v1/comments/<id>
```

gateway handler 默认连接 `APP_COMMENT_GRPC_TARGET`，没有设置时使用生成服务端口 `127.0.0.1:9103`。因此只要按顺序启动即可：

```bash
make run-comment
make run-gateway
```

## 11. 发布公共包给其他项目使用

如果脚手架仓库地址是：

```text
github.com/BwCloudWeGo/bw-cli
```

其他项目可以直接引入公共组件：

```bash
go get github.com/BwCloudWeGo/bw-cli/pkg/logger
go get github.com/BwCloudWeGo/bw-cli/pkg/mysqlx
go get github.com/BwCloudWeGo/bw-cli/pkg/postgresx
go get github.com/BwCloudWeGo/bw-cli/pkg/mongox
go get github.com/BwCloudWeGo/bw-cli/pkg/redisx
go get github.com/BwCloudWeGo/bw-cli/pkg/esx
go get github.com/BwCloudWeGo/bw-cli/pkg/kafkax
go get github.com/BwCloudWeGo/bw-cli/pkg/middleware
go get github.com/BwCloudWeGo/bw-cli/pkg/filex
```

示例：

```go
package main

import (
    "github.com/BwCloudWeGo/bw-cli/pkg/mysqlx"
)

func main() {
    cfg := mysqlx.DefaultConfig()
    cfg.DSN = "user:pass@tcp(mysql.example.com:3306)/app?charset=utf8mb4&parseTime=True&loc=Local"

    db, err := mysqlx.Open(cfg)
    if err != nil {
        panic(err)
    }

    _ = db
}
```

如果多个项目长期共用，建议把这些基础包拆到独立仓库，例如：

```text
github.com/your-org/go-kit/logger
github.com/your-org/go-kit/mysqlx
github.com/your-org/go-kit/postgresx
github.com/your-org/go-kit/mongox
github.com/your-org/go-kit/redisx
```

## 12. 常见问题

### 12.1 bw-cli: command not found

确认 Go bin 目录在 `PATH` 中：

```bash
go env GOPATH
```

如果输出是 `/Users/you/go`，把下面内容加入 shell 配置：

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

然后重新打开终端。

### 12.2 go install 远程安装失败

检查：

1. Git 仓库是否可访问。
2. `go.mod` 的 module 是否等于真实仓库路径。
3. 命令是否带了 `@latest`。

正确形式：

```bash
go install github.com/BwCloudWeGo/bw-cli/cmd/bw-cli@latest
```

### 12.3 生成项目后 protobuf panic

正常情况下不会出现。`bw-cli` 会跳过 `*.pb.go` 的 module 替换，并改写 `.proto` 的 `go_package`。如果你手动替换过生成文件，执行：

```bash
make proto
make test
```

重新生成即可。

### 12.4 演示项目 gateway 调用 user/note 失败

这个问题只适用于 `bw-cli demo` 生成的演示项目。`bw-cli new` 生成的干净项目默认没有 user/note 服务。

检查三个进程是否都启动：

```bash
make run-user
make run-note
make run-gateway
```

检查 `configs/config.yaml`：

```yaml
grpc:
  user_target: 127.0.0.1:9001
  note_target: 127.0.0.1:9002
```

### 12.5 JWT 返回 invalid token

检查生成 token 和验证 token 使用的是同一个 secret：

```bash
echo $APP_MIDDLEWARE_JWT_SECRET
```
