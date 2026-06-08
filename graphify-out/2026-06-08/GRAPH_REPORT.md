# Graph Report - .  (2026-06-08)

## Corpus Check
- 129 files · ~67,300 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1441 nodes · 2640 edges · 77 communities (53 shown, 24 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 239 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_CLI Scaffolding|CLI Scaffolding]]
- [[_COMMUNITY_App Error Handling|App Error Handling]]
- [[_COMMUNITY_Core Configuration|Core Configuration]]
- [[_COMMUNITY_Kafka Messaging|Kafka Messaging]]
- [[_COMMUNITY_Note Protobuf|Note Protobuf]]
- [[_COMMUNITY_Config Loading|Config Loading]]
- [[_COMMUNITY_Note Domain|Note Domain]]
- [[_COMMUNITY_HTTP Gateway|HTTP Gateway]]
- [[_COMMUNITY_Elasticsearch Search|Elasticsearch Search]]
- [[_COMMUNITY_User Protobuf|User Protobuf]]
- [[_COMMUNITY_Generic API Types|Generic API Types]]
- [[_COMMUNITY_Alipay Client|Alipay Client]]
- [[_COMMUNITY_Order gRPC Client|Order gRPC Client]]
- [[_COMMUNITY_Note gRPC Client|Note gRPC Client]]
- [[_COMMUNITY_Mongo Document Store|Mongo Document Store]]
- [[_COMMUNITY_User Domain|User Domain]]
- [[_COMMUNITY_Graphify Skill|Graphify Skill]]
- [[_COMMUNITY_Mongo Collection|Mongo Collection]]
- [[_COMMUNITY_Order Domain|Order Domain]]
- [[_COMMUNITY_User gRPC Service|User gRPC Service]]
- [[_COMMUNITY_Project Documentation|Project Documentation]]
- [[_COMMUNITY_Mongo Collection Tests|Mongo Collection Tests]]
- [[_COMMUNITY_File Upload Utilities|File Upload Utilities]]
- [[_COMMUNITY_Redis Locking|Redis Locking]]
- [[_COMMUNITY_Order gRPC Server|Order gRPC Server]]
- [[_COMMUNITY_CLI Main|CLI Main]]
- [[_COMMUNITY_User Service Tests|User Service Tests]]
- [[_COMMUNITY_Order Mongo Repository|Order Mongo Repository]]
- [[_COMMUNITY_Order Gorm Repository|Order Gorm Repository]]
- [[_COMMUNITY_Proto Generation|Proto Generation]]
- [[_COMMUNITY_Note gRPC Server|Note gRPC Server]]
- [[_COMMUNITY_Note Gorm Repository|Note Gorm Repository]]
- [[_COMMUNITY_Qiniu Storage|Qiniu Storage]]
- [[_COMMUNITY_Uploader Backend|Uploader Backend]]
- [[_COMMUNITY_Aliyun OSS Storage|Aliyun OSS Storage]]
- [[_COMMUNITY_Mongo Client|Mongo Client]]
- [[_COMMUNITY_MinIO Storage|MinIO Storage]]
- [[_COMMUNITY_Order Service Tests|Order Service Tests]]
- [[_COMMUNITY_Time Utilities|Time Utilities]]
- [[_COMMUNITY_Gorm Database|Gorm Database]]
- [[_COMMUNITY_gRPC Interceptors|gRPC Interceptors]]
- [[_COMMUNITY_Nacos Config|Nacos Config]]
- [[_COMMUNITY_Note Service Tests|Note Service Tests]]
- [[_COMMUNITY_Order Response Proto|Order Response Proto]]
- [[_COMMUNITY_Tencent COS Backend|Tencent COS Backend]]
- [[_COMMUNITY_Order List Proto|Order List Proto]]
- [[_COMMUNITY_Order Update Proto|Order Update Proto]]
- [[_COMMUNITY_Order Create Proto|Order Create Proto]]
- [[_COMMUNITY_Order Delete Proto|Order Delete Proto]]
- [[_COMMUNITY_MySQL Client|MySQL Client]]
- [[_COMMUNITY_Postgres Client|Postgres Client]]
- [[_COMMUNITY_Order List Request|Order List Request]]
- [[_COMMUNITY_Proto Reflection|Proto Reflection]]
- [[_COMMUNITY_Order Delete Response|Order Delete Response]]
- [[_COMMUNITY_Note Persistence Models|Note Persistence Models]]
- [[_COMMUNITY_Order Persistence Models|Order Persistence Models]]
- [[_COMMUNITY_Order Proto Descriptors|Order Proto Descriptors]]
- [[_COMMUNITY_Order Get Proto|Order Get Proto]]
- [[_COMMUNITY_Mongo Client Tests|Mongo Client Tests]]
- [[_COMMUNITY_Order Commands|Order Commands]]
- [[_COMMUNITY_Note Request Tests|Note Request Tests]]
- [[_COMMUNITY_User Persistence Models|User Persistence Models]]
- [[_COMMUNITY_Order HTTP Requests|Order HTTP Requests]]
- [[_COMMUNITY_Hook Configuration|Hook Configuration]]
- [[_COMMUNITY_Note Commands|Note Commands]]
- [[_COMMUNITY_User Commands|User Commands]]
- [[_COMMUNITY_User Request Tests|User Request Tests]]
- [[_COMMUNITY_Mongo Options Tests|Mongo Options Tests]]
- [[_COMMUNITY_Observability Register|Observability Register]]
- [[_COMMUNITY_Note HTTP Requests|Note HTTP Requests]]
- [[_COMMUNITY_User HTTP Requests|User HTTP Requests]]
- [[_COMMUNITY_Order Proto Init|Order Proto Init]]
- [[_COMMUNITY_Agent Graphify Rules|Agent Graphify Rules]]
- [[_COMMUNITY_Note Repository Interface|Note Repository Interface]]
- [[_COMMUNITY_Order Repository Interface|Order Repository Interface]]
- [[_COMMUNITY_User Repository Interface|User Repository Interface]]

## God Nodes (most connected - your core abstractions)
1. `writeFile()` - 31 edges
2. `Config` - 26 edges
3. `exists()` - 25 edges
4. `FromGRPC()` - 19 edges
5. `NoteResponse` - 18 edges
6. `Config` - 18 edges
7. `serviceTemplateData` - 17 edges
8. `PublishNoteRequest` - 16 edges
9. `OrderResponse` - 16 edges
10. `AddService()` - 16 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `RegisterNoteServiceServer()`  [INFERRED]
  cmd/note/main.go → api/gen/note/v1/note_grpc.pb.go
- `New()` --calls--> `NewOrderServiceClient()`  [INFERRED]
  internal/gateway/client/clients.go → api/gen/order/v1/order_grpc.pb.go
- `main()` --calls--> `RegisterOrderServiceServer()`  [INFERRED]
  cmd/order/main.go → api/gen/order/v1/order_grpc.pb.go
- `New()` --calls--> `NewUserServiceClient()`  [INFERRED]
  internal/gateway/client/clients.go → api/gen/user/v1/user_grpc.pb.go
- `main()` --calls--> `RegisterUserServiceServer()`  [INFERRED]
  cmd/user/main.go → api/gen/user/v1/user_grpc.pb.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Layered Service Flow** — docs_architecture_ddd_layering, docs_mongo_call_examples_pkg_mongox_documentstore, services_order_order_service [INFERRED 0.85]
- **Configuration Source Management** — configs_config_local_business_config, configs_nacos_nacos_switch, docs_nacos_nacos_config_center [INFERRED 0.85]
- **External Integration Guides** — docs_alipay_pkg_alipayx, docs_elasticsearch_pkg_esx_searcher, docs_mongo_call_examples_pkg_mongox_documentstore [INFERRED 0.75]
- **Graph Update Automation Modes** — references_add_watch_watch_mode, references_hooks_post_commit_auto_rebuild, references_update_incremental_reextraction [INFERRED 0.85]
- **Query Navigation Flows** — graphify_skill_existing_graph_query_flow, graphify_skill_query_path_and_explain, references_query_bfs_traversal, references_query_dfs_traversal [INFERRED 0.75]
- **Semantic Extraction Pipeline** — graphify_skill_semantic_extraction, references_extraction_spec_semantic_fragment_schema, references_transcribe_whisper_transcription [INFERRED 0.75]

## Communities (77 total, 24 thin omitted)

### Community 0 - "CLI Scaffolding"
Cohesion: 0.06
Nodes (97): writeFile(), DirEntry, DB, T, Rows, DeleteServiceOptions, InitOptions, cleanArchitectureDoc() (+89 more)

### Community 1 - "App Error Handling"
Cohesion: 0.06
Nodes (58): Code, ErrorBody, AppError, As(), Conflict(), FromGRPC(), GRPCCode(), HTTPStatus() (+50 more)

### Community 2 - "Core Configuration"
Cohesion: 0.05
Nodes (50): Config, Logger, Server, Context, Logger, Server, Config, Context (+42 more)

### Community 3 - "Kafka Messaging"
Cohesion: 0.06
Nodes (44): Compression, ConsumerConfig, Dialer, captureWriter, Config, Consumer, ConsumerConfig, Handler (+36 more)

### Community 4 - "Note Protobuf"
Cohesion: 0.05
Nodes (11): Message, MessageState, SizeCache, UnknownFields, CreateNoteRequest, GetNoteRequest, file_note_v1_note_proto_init(), file_note_v1_note_proto_rawDescGZIP() (+3 more)

### Community 5 - "Config Loading"
Cohesion: 0.07
Nodes (40): AppConfig, Config, decodeHook(), isConfigFileNotFound(), Load(), loadFromLocal(), loadFromNacos(), loadNacosConfig() (+32 more)

### Community 6 - "Note Domain"
Cohesion: 0.07
Nodes (37): CreateNoteCommand, DocumentSaverFinder, FromNote(), NoteDTO, Note, NewNote(), NoteStatusFromCode(), NoteStatus (+29 more)

### Community 7 - "HTTP Gateway"
Cohesion: 0.06
Nodes (36): Engine, RouterGroup, RouterGroup, Clients, Engine, Logger, MiddlewareConfig, JWTConfig (+28 more)

### Community 8 - "Elasticsearch Search"
Cohesion: 0.11
Nodes (36): Aggregation, AggregationRequest, clientSearchExecutor, Config, boolQuery(), BuildAggregationBody(), BuildSearchBody(), DateHistogramAggregation() (+28 more)

### Community 9 - "User Protobuf"
Cohesion: 0.07
Nodes (11): Message, MessageState, SizeCache, UnknownFields, GetUserRequest, LoginRequest, RegisterRequest, file_user_v1_user_proto_init() (+3 more)

### Community 10 - "Generic API Types"
Cohesion: 0.09
Nodes (31): Context, CreateNoteRequest, GetNoteRequest, GetUserRequest, LoginRequest, NoteResponse, PublishNoteRequest, RegisterRequest (+23 more)

### Community 11 - "Alipay Client"
Cohesion: 0.12
Nodes (24): DefaultConfig(), ensureRefundSuccess(), firstNonEmpty(), hasAllCertPaths(), hasAnyCertPath(), NewClient(), normalizeConfig(), TestEnsureRefundSuccessAcceptsSuccessCode() (+16 more)

### Community 12 - "Order gRPC Client"
Cohesion: 0.12
Nodes (23): ClientConnInterface, Context, CreateOrderRequest, DeleteOrderRequest, DeleteOrderResponse, GetOrderRequest, ListOrdersRequest, ListOrdersResponse (+15 more)

### Community 13 - "Note gRPC Client"
Cohesion: 0.10
Nodes (25): ClientConnInterface, Context, CreateNoteRequest, GetNoteRequest, NoteResponse, PublishNoteRequest, ServiceRegistrar, UnaryServerInterceptor (+17 more)

### Community 14 - "Mongo Document Store"
Cohesion: 0.12
Nodes (20): Collection, CollectionNamed, NewDocumentStore(), newDocumentStoreWithCollection(), NewNamedDocumentStore(), TestDocumentStoreExposesUnderlyingCollection(), TestDocumentStoreForwardsCRUDToCollection(), TestDocumentStoreForwardsMissingDocumentError() (+12 more)

### Community 15 - "User Domain"
Cohesion: 0.11
Nodes (26): FromUser(), UserDTO, User, NewUser(), NormalizeAccount(), User, Time, Context (+18 more)

### Community 16 - "Graphify Skill"
Cohesion: 0.09
Nodes (29): Existing Graph Fast Path, Existing Graph Query Flow, graphify Skill, Query, Path, and Explain Flows, Semantic Extraction, Structural Extraction, Add a URL and Watch a Folder, graphify add (+21 more)

### Community 17 - "Mongo Collection"
Cohesion: 0.17
Nodes (16): Field, Collection, NewCollection(), newCollectionWithOperator(), normalizeFindErr(), Collection[T], updateResultFields(), collectionOperator (+8 more)

### Community 18 - "Order Domain"
Cohesion: 0.12
Nodes (20): CreateCommand, ListOrderDTO, formatTime(), FromOrder(), OrderDTO, Order, NewOrder(), Order (+12 more)

### Community 19 - "User gRPC Service"
Cohesion: 0.15
Nodes (17): ClientConnInterface, Context, GetUserRequest, LoginRequest, RegisterRequest, ServiceRegistrar, UnaryServerInterceptor, UserResponse (+9 more)

### Community 20 - "Project Documentation"
Cohesion: 0.12
Nodes (26): Application Configuration, Local Business Configuration, Nacos Switch Configuration, Nacos Source Switch, Docker Compose Stack, Local Runtime Stack, Alipay Integration Guide, pkg/alipayx (+18 more)

### Community 21 - "Mongo Collection Tests"
Cohesion: 0.13
Nodes (17): Cursor, FindOptions, Lister, TestCollectionFindByIDDecodesDocument(), TestCollectionFindByIDMapsMissingDocument(), TestCollectionFindManyDecodesDocuments(), TestCollectionForwardsWriteErrors(), TestCollectionUpsertByIDUsesReplaceWithUpsert() (+9 more)

### Community 22 - "File Upload Utilities"
Cohesion: 0.16
Nodes (23): containsFold(), contentTypeAllowed(), DefaultAllowedContentTypes(), DefaultAllowedExtensions(), DefaultConfig(), DetectContentType(), maxSizeBytes(), NewObjectKey() (+15 more)

### Community 23 - "Redis Locking"
Cohesion: 0.18
Nodes (14): Lock, LockConfig, Client, Context, Duration, Config, Lock, LockConfig (+6 more)

### Community 24 - "Order gRPC Server"
Cohesion: 0.17
Nodes (17): mapOrderError(), Context, CreateOrderRequest, DeleteOrderRequest, DeleteOrderResponse, GetOrderRequest, Server, NewServer() (+9 more)

### Community 25 - "CLI Main"
Cohesion: 0.22
Nodes (17): main(), parseDeleteServiceOptions(), parseGenerateOptions(), parseServiceOptions(), runDeleteService(), runGenerate(), runService(), splitTargetArg() (+9 more)

### Community 26 - "User Service Tests"
Cohesion: 0.20
Nodes (12): Context, Repository, Service, T, User, memoryUserRepo, plainHasher, newMemoryUserRepo() (+4 more)

### Community 27 - "Order Mongo Repository"
Cohesion: 0.24
Nodes (11): DocumentStore, Context, Database, Logger, Order, NewMongoRepository(), MongoRepository, Time (+3 more)

### Community 28 - "Order Gorm Repository"
Cohesion: 0.25
Nodes (11): Context, DB, Logger, Order, AutoMigrate(), NewGormRepository(), GormRepository, Time (+3 more)

### Community 29 - "Proto Generation"
Cohesion: 0.23
Nodes (15): cleanPathEntries(), collectProtoFiles(), getenvDefault(), goEnv(), main(), prependPath(), protoConfigFromEnv(), run() (+7 more)

### Community 30 - "Note gRPC Server"
Cohesion: 0.24
Nodes (13): mapNoteError(), Context, CreateNoteRequest, GetNoteRequest, Server, NewServer(), toProto(), Logger (+5 more)

### Community 31 - "Note Gorm Repository"
Cohesion: 0.25
Nodes (11): Context, DB, Logger, Note, AutoMigrate(), NewGormRepository(), GormRepository, Time (+3 more)

### Community 32 - "Qiniu Storage"
Cohesion: 0.15
Nodes (10): newQiniuBackend(), qiniuMetadata(), qiniuBackend, FormUploader, Mac, backend, Config, Context (+2 more)

### Community 33 - "Uploader Backend"
Cohesion: 0.19
Nodes (12): backend, Config, newTencentCOSBackend(), newBackend(), NewUploader(), Uploader, backend, Config (+4 more)

### Community 34 - "Aliyun OSS Storage"
Cohesion: 0.18
Nodes (8): Bucket, newOSSBackend(), ossBackend, backend, Config, Context, OSSConfig, preparedUpload

### Community 35 - "Mongo Client"
Cohesion: 0.29
Nodes (11): ClientOptions, Config, clientOptions(), Database(), DefaultConfig(), NewClient(), Ping(), Client (+3 more)

### Community 36 - "MinIO Storage"
Cohesion: 0.17
Nodes (8): newMinIOBackend(), minioBackend, backend, Client, Config, Context, MinIOConfig, preparedUpload

### Community 37 - "Order Service Tests"
Cohesion: 0.33
Nodes (7): Context, Order, T, fakeRepository, newFakeRepository(), TestNewService(), TestServiceCRUD()

### Community 38 - "Time Utilities"
Cohesion: 0.38
Nodes (11): Location, Time, Age(), AgeAt(), FormatDateTime(), formatRelative(), hasNotReachedBirthdayThisYear(), isLeapYear() (+3 more)

### Community 39 - "Gorm Database"
Cohesion: 0.29
Nodes (10): ensureDir(), Open(), ToMySQLConfig(), ToPostgreSQLConfig(), DatabaseConfig, MySQLConfig, Config, DB (+2 more)

### Community 40 - "gRPC Interceptors"
Cohesion: 0.31
Nodes (10): errorCode(), firstMetadata(), peerAddress(), RequestIDFromContext(), UnaryClientInterceptor(), UnaryServerInterceptor(), Context, Logger (+2 more)

### Community 41 - "Nacos Config"
Cohesion: 0.36
Nodes (9): IConfigClient, Config, DefaultConfig(), LoadConfig(), NewConfigClient(), TestLoadConfigReturnsEmptyWhenDisabled(), TestWithDefaultsKeepsNacosDisabledAndFillsConnectionDefaults(), WithDefaults() (+1 more)

### Community 42 - "Note Service Tests"
Cohesion: 0.36
Nodes (8): Context, Note, T, memoryNoteRepo, newMemoryNoteRepo(), TestCreateNote(), TestCreateNoteRequiresAuthorTitleAndContent(), TestPublishSubmittedCreatesPublishedNote()

### Community 44 - "Tencent COS Backend"
Cohesion: 0.25
Nodes (5): cosBackend, Client, Context, preparedUpload, TencentCOSConfig

### Community 49 - "MySQL Client"
Cohesion: 0.38
Nodes (6): Config, DefaultConfig(), Open(), DB, Duration, LogLevel

### Community 50 - "Postgres Client"
Cohesion: 0.38
Nodes (6): DB, Duration, LogLevel, Config, DefaultConfig(), Open()

### Community 54 - "Note Persistence Models"
Cohesion: 0.40
Nodes (3): Time, NoteDocument, NoteModel

### Community 55 - "Order Persistence Models"
Cohesion: 0.40
Nodes (3): Time, OrderDocument, OrderModel

### Community 58 - "Mongo Client Tests"
Cohesion: 0.60
Nodes (4): TestDatabaseReturnsConfiguredDatabase(), TestDefaultConfigUsesLocalDevelopmentValues(), TestNewClientFallsBackToDefaultURI(), T

### Community 59 - "Order Commands"
Cohesion: 0.50
Nodes (3): CreateCommand, ListCommand, UpdateCommand

### Community 60 - "Note Request Tests"
Cohesion: 0.67
Nodes (3): T, TestNoteRequestsAreDeclaredOutsideHandlers(), TestPublishNoteRequestBindsExtendedFields()

### Community 62 - "Order HTTP Requests"
Cohesion: 0.50
Nodes (3): CreateOrderRequest, ListOrderRequest, UpdateOrderRequest

## Knowledge Gaps
- **243 isolated node(s):** `PreToolUse`, `UnsafeNoteServiceServer`, `ServiceRegistrar`, `UnsafeOrderServiceServer`, `ServiceRegistrar` (+238 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **24 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Load()` connect `Config Loading` to `CLI Scaffolding`, `Core Configuration`?**
  _High betweenness centrality (0.075) - this node is a cross-community bridge._
- **Why does `setDefaults()` connect `Config Loading` to `Generic API Types`, `HTTP Gateway`?**
  _High betweenness centrality (0.067) - this node is a cross-community bridge._
- **Are the 24 inferred relationships involving `writeFile()` (e.g. with `TestServiceHelpersUseConfiguredValues()` and `TestServiceTargetUsesConfiguredPortWhenTargetMissing()`) actually correct?**
  _`writeFile()` has 24 INFERRED edges - model-reasoned connections that need verification._
- **Are the 19 inferred relationships involving `exists()` (e.g. with `addServiceConfig()` and `addServiceMakeTarget()`) actually correct?**
  _`exists()` has 19 INFERRED edges - model-reasoned connections that need verification._
- **Are the 13 inferred relationships involving `FromGRPC()` (e.g. with `errorCode()` and `.Create()`) actually correct?**
  _`FromGRPC()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **What connects `PreToolUse`, `UnsafeNoteServiceServer`, `ServiceRegistrar` to the rest of the system?**
  _247 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `CLI Scaffolding` be split into smaller, more focused modules?**
  _Cohesion score 0.058787878787878785 - nodes in this community are weakly interconnected._