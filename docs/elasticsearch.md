# Elasticsearch 调用示例

本文档说明如何使用 `pkg/esx` 创建 Elasticsearch v7 client，并调用模糊搜索、高亮和聚合查询。底层 SDK 使用 `github.com/elastic/go-elasticsearch/v7`。本地或无认证集群只需要配置 `addresses`，其它认证参数保留为可选项。

## 配置

```yaml
elasticsearch:
  addresses:
    - http://127.0.0.1:9200
  username: ""
  password: ""
  cloud_id: ""
  api_key: ""
```

最小配置只需要：

```yaml
elasticsearch:
  addresses:
    - http://127.0.0.1:9200
```

## 初始化

```go
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()

client, err := esx.NewClient(cfg.Elasticsearch)
if err != nil {
    panic(err)
}
searcher := esx.NewSearcherFromClient(client)
```

`NewClient` 返回官方 v7 client。需要调用未封装的接口时，直接使用 `client` 即可。

## 模糊搜索和高亮

```go
result, err := searcher.FuzzySearch(ctx, esx.FuzzySearchRequest{
    Index:   "notes",
    Keyword: "golang",
    Fields:  []string{"title^2", "content"},
    From:    0,
    Size:    20,
    Filters: []esx.Filter{
        esx.TermFilter("status", "published"),
        esx.RangeFilter("created_at", map[string]any{
            "gte": "2026-01-01",
        }),
    },
    Sort: []esx.Sort{
        esx.SortField("created_at", "desc"),
    },
    Highlight: esx.HighlightConfig{
        Fields:            []string{"title", "content"},
        PreTags:           []string{"<mark>"},
        PostTags:          []string{"</mark>"},
        FragmentSize:      120,
        NumberOfFragments: 2,
    },
})
if err != nil {
    return err
}

for _, hit := range result.Hits {
    var note struct {
        Title   string `json:"title"`
        Content string `json:"content"`
    }
    if err := json.Unmarshal(hit.Source, &note); err != nil {
        return err
    }
    fmt.Println(hit.ID, note.Title, hit.Highlight["title"])
}
```

封装生成的核心 DSL 是 `multi_match`：

```json
{
  "query": {
    "bool": {
      "must": {
        "multi_match": {
          "query": "golang",
          "fields": ["title^2", "content"],
          "type": "best_fields",
          "operator": "or",
          "fuzziness": "AUTO"
        }
      },
      "filter": [
        {"term": {"status": "published"}}
      ]
    }
  }
}
```

## 聚合查询

```go
result, err := searcher.Aggregate(ctx, esx.AggregationRequest{
    Index: "notes",
    Filters: []esx.Filter{
        esx.TermFilter("status", "published"),
    },
    Aggregations: map[string]esx.Aggregation{
        "by_author": esx.TermsAggregation("author_id", 10),
        "by_day":    esx.DateHistogramAggregation("created_at", "day"),
        "by_score": esx.RangeAggregation("score", []map[string]any{
            {"to": 60},
            {"from": 60, "to": 90},
            {"from": 90},
        }),
    },
})
if err != nil {
    return err
}

var aggs map[string]any
if err := json.Unmarshal(result.Aggregations, &aggs); err != nil {
    return err
}
```

如果业务需要更复杂的聚合，`Aggregation` 本质是 `map[string]any`，可以直接透传 Elasticsearch DSL：

```go
"nested_tags": esx.Aggregation{
    "nested": map[string]any{"path": "tags"},
    "aggs": map[string]any{
        "top_tags": map[string]any{
            "terms": map[string]any{"field": "tags.name.keyword", "size": 20},
        },
    },
}
```

## MySQL 同步到 ES

常见同步方式有两种：

1. **事件同步**：MySQL 写入成功后发布 Kafka/Redis Stream/内部事件，由消费者写 ES。实时性较好，链路稍复杂。
2. **增量轮询**：定时按 `updated_at` 和 `id` 游标扫描 MySQL，批量写 ES。实现简单，适合后台补偿和低频索引。

下面示例使用 Gorm 增量轮询。

### 数据结构

```go
type Note struct {
    ID        string
    AuthorID  string
    Title     string
    Content   string
    Status    string
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt
}

type NoteDocument struct {
    ID        string    `json:"id"`
    AuthorID  string    `json:"author_id"`
    Title     string    `json:"title"`
    Content   string    `json:"content"`
    Status    string    `json:"status"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 增量扫描

```go
func LoadChangedNotes(ctx context.Context, db *gorm.DB, cursorTime time.Time, cursorID string, limit int) ([]Note, error) {
    var notes []Note
    err := db.WithContext(ctx).
        Where("updated_at > ? OR (updated_at = ? AND id > ?)", cursorTime, cursorTime, cursorID).
        Order("updated_at ASC").
        Order("id ASC").
        Limit(limit).
        Find(&notes).Error
    return notes, err
}
```

### Bulk 写入 ES

```go
func SyncNotesToES(ctx context.Context, client *elasticsearch.Client, notes []Note) error {
    var buf bytes.Buffer
    encoder := json.NewEncoder(&buf)

    for _, note := range notes {
        if note.DeletedAt.Valid {
            if err := encoder.Encode(map[string]any{
                "delete": map[string]any{"_index": "notes", "_id": note.ID},
            }); err != nil {
                return err
            }
            continue
        }

        doc := NoteDocument{
            ID:        note.ID,
            AuthorID:  note.AuthorID,
            Title:     note.Title,
            Content:   note.Content,
            Status:    note.Status,
            UpdatedAt: note.UpdatedAt,
        }
        if err := encoder.Encode(map[string]any{
            "index": map[string]any{"_index": "notes", "_id": note.ID},
        }); err != nil {
            return err
        }
        if err := encoder.Encode(doc); err != nil {
            return err
        }
    }

    if buf.Len() == 0 {
        return nil
    }
    res, err := client.Bulk(bytes.NewReader(buf.Bytes()), client.Bulk.WithContext(ctx))
    if err != nil {
        return err
    }
    defer res.Body.Close()
    if res.IsError() {
        data, _ := io.ReadAll(res.Body)
        return fmt.Errorf("bulk index notes failed: %s", data)
    }
    return nil
}
```

### 游标保存

每批同步成功后，把最后一条记录的 `updated_at` 和 `id` 保存到 MySQL、Redis 或配置表：

```go
func AdvanceCursor(notes []Note) (time.Time, string) {
    if len(notes) == 0 {
        return time.Time{}, ""
    }
    last := notes[len(notes)-1]
    return last.UpdatedAt, last.ID
}
```

生产环境建议同时保留事件同步和增量轮询：事件负责实时写入，轮询负责补偿漏消息和重建索引。
