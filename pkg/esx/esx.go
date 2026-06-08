package esx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

// Config 控制 Elasticsearch 客户端连接设置。
type Config struct {
	Addresses []string `mapstructure:"addresses" yaml:"addresses"`
	Username  string   `mapstructure:"username" yaml:"username"`
	Password  string   `mapstructure:"password" yaml:"password"`
	CloudID   string   `mapstructure:"cloud_id" yaml:"cloud_id"`
	APIKey    string   `mapstructure:"api_key" yaml:"api_key"`
}

// DefaultConfig 返回本地开发可用的 Elasticsearch 默认配置。
func DefaultConfig() Config {
	return Config{
		Addresses: []string{"http://127.0.0.1:9200"},
	}
}

// NewClient 根据配置创建 Elasticsearch v7 客户端。
func NewClient(cfg Config) (*elasticsearch.Client, error) {
	if len(cfg.Addresses) == 0 && cfg.CloudID == "" {
		cfg.Addresses = DefaultConfig().Addresses
	}
	return elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		CloudID:   cfg.CloudID,
		APIKey:    cfg.APIKey,
	})
}

// SearchExecutor 是搜索场景需要的官方客户端最小接口子集。
type SearchExecutor interface {
	Search(ctx context.Context, index []string, body io.Reader) (*esapi.Response, error)
}

type clientSearchExecutor struct {
	client *elasticsearch.Client
}

// Search 通过官方客户端执行一次原始搜索请求。
func (e clientSearchExecutor) Search(ctx context.Context, index []string, body io.Reader) (*esapi.Response, error) {
	return e.client.Search(
		e.client.Search.WithContext(ctx),
		e.client.Search.WithIndex(index...),
		e.client.Search.WithBody(body),
	)
}

// Searcher 封装常见 Elasticsearch 搜索模式，但不隐藏原始 DSL。
type Searcher struct {
	executor SearchExecutor
}

// NewSearcher 创建可复用的搜索辅助对象。
func NewSearcher(executor SearchExecutor) *Searcher {
	return &Searcher{executor: executor}
}

// NewSearcherFromClient 基于官方 v7 客户端创建搜索辅助对象。
func NewSearcherFromClient(client *elasticsearch.Client) *Searcher {
	return NewSearcher(clientSearchExecutor{client: client})
}

// Filter 表示一个 Elasticsearch 过滤子句。
type Filter map[string]any

// Sort 表示一个 Elasticsearch 排序子句。
type Sort map[string]any

// Aggregation 表示一个具名聚合定义。
type Aggregation map[string]any

// HighlightConfig 控制 Elasticsearch 高亮渲染。
type HighlightConfig struct {
	Fields            []string
	PreTags           []string
	PostTags          []string
	FragmentSize      int
	NumberOfFragments int
}

// FuzzySearchRequest 描述多字段模糊搜索请求。
type FuzzySearchRequest struct {
	Index     string
	Keyword   string
	Fields    []string
	From      int
	Size      int
	Filters   []Filter
	Sort      []Sort
	Highlight HighlightConfig
}

// AggregationRequest 描述只执行聚合的查询。
type AggregationRequest struct {
	Index        string
	Filters      []Filter
	Aggregations map[string]Aggregation
}

// SearchRequest 允许调用方通过封装传入原始 Elasticsearch DSL。
type SearchRequest struct {
	Index string
	Body  map[string]any
}

// SearchResult 是 Searcher 返回的规范化响应。
type SearchResult struct {
	Total        int64
	Hits         []Hit
	Aggregations json.RawMessage
	Raw          json.RawMessage
}

// Hit 表示一条 Elasticsearch 命中结果，包含 source 和高亮片段。
type Hit struct {
	ID        string
	Index     string
	Score     float64
	Source    json.RawMessage
	Highlight map[string][]string
}

// TermFilter 创建 term 过滤子句。
func TermFilter(field string, value any) Filter {
	return Filter{"term": map[string]any{field: value}}
}

// RangeFilter 创建 range 过滤子句。
func RangeFilter(field string, constraints map[string]any) Filter {
	return Filter{"range": map[string]any{field: constraints}}
}

// SortField 创建一个字段排序子句。
func SortField(field string, order string) Sort {
	if strings.TrimSpace(order) == "" {
		order = "asc"
	}
	return Sort{field: map[string]any{"order": strings.ToLower(strings.TrimSpace(order))}}
}

// TermsAggregation 创建 terms 聚合。
func TermsAggregation(field string, size int) Aggregation {
	body := map[string]any{"field": field}
	if size > 0 {
		body["size"] = size
	}
	return Aggregation{"terms": body}
}

// DateHistogramAggregation 创建日历间隔日期直方图聚合。
func DateHistogramAggregation(field string, interval string) Aggregation {
	return Aggregation{"date_histogram": map[string]any{"field": field, "calendar_interval": interval}}
}

// RangeAggregation 创建数值或日期范围聚合。
func RangeAggregation(field string, ranges []map[string]any) Aggregation {
	return Aggregation{"range": map[string]any{"field": field, "ranges": ranges}}
}

// BuildSearchBody 为模糊搜索请求构建 Elasticsearch JSON body。
func BuildSearchBody(req FuzzySearchRequest) (io.Reader, error) {
	if strings.TrimSpace(req.Keyword) == "" {
		return nil, fmt.Errorf("es keyword is required")
	}
	if len(req.Fields) == 0 {
		return nil, fmt.Errorf("es fuzzy fields are required")
	}
	if req.Size <= 0 {
		req.Size = 20
	}

	body := map[string]any{
		"from": req.From,
		"size": req.Size,
		"query": boolQuery(map[string]any{"multi_match": map[string]any{
			"query":     req.Keyword,
			"fields":    req.Fields,
			"type":      "best_fields",
			"operator":  "or",
			"fuzziness": "AUTO",
		}}, req.Filters),
	}
	if len(req.Sort) > 0 {
		body["sort"] = req.Sort
	}
	if highlight := highlightBody(req.Highlight); len(highlight) > 0 {
		body["highlight"] = highlight
	}
	return encodeBody(body)
}

// BuildAggregationBody 为聚合查询构建 Elasticsearch JSON body。
func BuildAggregationBody(req AggregationRequest) (io.Reader, error) {
	if len(req.Aggregations) == 0 {
		return nil, fmt.Errorf("es aggregations are required")
	}
	body := map[string]any{
		"size": 0,
		"aggs": req.Aggregations,
	}
	if len(req.Filters) > 0 {
		body["query"] = boolQuery(nil, req.Filters)
	}
	return encodeBody(body)
}

// FuzzySearch 执行模糊搜索，并解析命中、高亮和聚合结果。
func (s *Searcher) FuzzySearch(ctx context.Context, req FuzzySearchRequest) (*SearchResult, error) {
	body, err := BuildSearchBody(req)
	if err != nil {
		return nil, err
	}
	return s.doSearch(ctx, req.Index, body)
}

// Aggregate 执行聚合查询并解析聚合桶。
func (s *Searcher) Aggregate(ctx context.Context, req AggregationRequest) (*SearchResult, error) {
	body, err := BuildAggregationBody(req)
	if err != nil {
		return nil, err
	}
	return s.doSearch(ctx, req.Index, body)
}

// Search 执行原始 Elasticsearch 查询 body，并解析规范化响应。
func (s *Searcher) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if len(req.Body) == 0 {
		return nil, fmt.Errorf("es search body is required")
	}
	body, err := encodeBody(req.Body)
	if err != nil {
		return nil, err
	}
	return s.doSearch(ctx, req.Index, body)
}

func (s *Searcher) doSearch(ctx context.Context, index string, body io.Reader) (*SearchResult, error) {
	if s == nil || s.executor == nil {
		return nil, fmt.Errorf("es search executor is required")
	}
	if strings.TrimSpace(index) == "" {
		return nil, fmt.Errorf("es index is required")
	}
	resp, err := s.executor.Search(ctx, []string{index}, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("es search failed: status=%d body=%s", resp.StatusCode, string(data))
	}
	return parseSearchResult(data)
}

func parseSearchResult(data []byte) (*SearchResult, error) {
	var raw struct {
		Hits struct {
			Total any `json:"total"`
			Hits  []struct {
				ID        string              `json:"_id"`
				Index     string              `json:"_index"`
				Score     float64             `json:"_score"`
				Source    json.RawMessage     `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations json.RawMessage `json:"aggregations"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	result := &SearchResult{
		Total:        totalHits(raw.Hits.Total),
		Hits:         make([]Hit, 0, len(raw.Hits.Hits)),
		Aggregations: raw.Aggregations,
		Raw:          append(json.RawMessage(nil), data...),
	}
	for _, hit := range raw.Hits.Hits {
		result.Hits = append(result.Hits, Hit{
			ID:        hit.ID,
			Index:     hit.Index,
			Score:     hit.Score,
			Source:    hit.Source,
			Highlight: hit.Highlight,
		})
	}
	return result, nil
}

func totalHits(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case map[string]any:
		if n, ok := typed["value"].(float64); ok {
			return int64(n)
		}
	}
	return 0
}

func boolQuery(must map[string]any, filters []Filter) map[string]any {
	boolBody := map[string]any{}
	if must != nil {
		boolBody["must"] = must
	}
	if len(filters) > 0 {
		boolBody["filter"] = filters
	}
	return map[string]any{"bool": boolBody}
}

func highlightBody(cfg HighlightConfig) map[string]any {
	if len(cfg.Fields) == 0 {
		return nil
	}
	out := map[string]any{
		"fields": map[string]any{},
	}
	fields := out["fields"].(map[string]any)
	for _, field := range cfg.Fields {
		field = strings.TrimSpace(field)
		if field != "" {
			fields[field] = map[string]any{}
		}
	}
	if len(fields) == 0 {
		return nil
	}
	preTags := cfg.PreTags
	if len(preTags) == 0 {
		preTags = []string{"<em>"}
	}
	postTags := cfg.PostTags
	if len(postTags) == 0 {
		postTags = []string{"</em>"}
	}
	out["pre_tags"] = preTags
	out["post_tags"] = postTags
	if cfg.FragmentSize > 0 {
		out["fragment_size"] = cfg.FragmentSize
	}
	if cfg.NumberOfFragments > 0 {
		out["number_of_fragments"] = cfg.NumberOfFragments
	}
	return out
}

func encodeBody(body map[string]any) (io.Reader, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}
