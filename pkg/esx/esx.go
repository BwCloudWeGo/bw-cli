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

// Config controls Elasticsearch client connection settings.
type Config struct {
	Addresses []string `mapstructure:"addresses" yaml:"addresses"`
	Username  string   `mapstructure:"username" yaml:"username"`
	Password  string   `mapstructure:"password" yaml:"password"`
	CloudID   string   `mapstructure:"cloud_id" yaml:"cloud_id"`
	APIKey    string   `mapstructure:"api_key" yaml:"api_key"`
}

// DefaultConfig returns local-development Elasticsearch defaults.
func DefaultConfig() Config {
	return Config{
		Addresses: []string{"http://127.0.0.1:9200"},
	}
}

// NewClient creates an Elasticsearch v7 client from configuration.
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

// SearchExecutor is the small subset of the official client needed for searches.
type SearchExecutor interface {
	Search(ctx context.Context, index []string, body io.Reader) (*esapi.Response, error)
}

type clientSearchExecutor struct {
	client *elasticsearch.Client
}

// Search executes one raw search request through the official client.
func (e clientSearchExecutor) Search(ctx context.Context, index []string, body io.Reader) (*esapi.Response, error) {
	return e.client.Search(
		e.client.Search.WithContext(ctx),
		e.client.Search.WithIndex(index...),
		e.client.Search.WithBody(body),
	)
}

// Searcher wraps common Elasticsearch search patterns without hiding the raw DSL.
type Searcher struct {
	executor SearchExecutor
}

// NewSearcher creates a reusable search helper.
func NewSearcher(executor SearchExecutor) *Searcher {
	return &Searcher{executor: executor}
}

// NewSearcherFromClient creates a search helper from the official v7 client.
func NewSearcherFromClient(client *elasticsearch.Client) *Searcher {
	return NewSearcher(clientSearchExecutor{client: client})
}

// Filter represents one Elasticsearch filter clause.
type Filter map[string]any

// Sort represents one Elasticsearch sort clause.
type Sort map[string]any

// Aggregation represents one named aggregation definition.
type Aggregation map[string]any

// HighlightConfig controls Elasticsearch highlight rendering.
type HighlightConfig struct {
	Fields            []string
	PreTags           []string
	PostTags          []string
	FragmentSize      int
	NumberOfFragments int
}

// FuzzySearchRequest describes a multi-field fuzzy search.
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

// AggregationRequest describes an aggregation-only query.
type AggregationRequest struct {
	Index        string
	Filters      []Filter
	Aggregations map[string]Aggregation
}

// SearchRequest allows callers to pass raw Elasticsearch DSL through the wrapper.
type SearchRequest struct {
	Index string
	Body  map[string]any
}

// SearchResult is the normalized response returned by Searcher.
type SearchResult struct {
	Total        int64
	Hits         []Hit
	Aggregations json.RawMessage
	Raw          json.RawMessage
}

// Hit is one Elasticsearch hit with source and highlight snippets.
type Hit struct {
	ID        string
	Index     string
	Score     float64
	Source    json.RawMessage
	Highlight map[string][]string
}

// TermFilter creates a term filter clause.
func TermFilter(field string, value any) Filter {
	return Filter{"term": map[string]any{field: value}}
}

// RangeFilter creates a range filter clause.
func RangeFilter(field string, constraints map[string]any) Filter {
	return Filter{"range": map[string]any{field: constraints}}
}

// SortField creates one field sort clause.
func SortField(field string, order string) Sort {
	if strings.TrimSpace(order) == "" {
		order = "asc"
	}
	return Sort{field: map[string]any{"order": strings.ToLower(strings.TrimSpace(order))}}
}

// TermsAggregation creates a terms aggregation.
func TermsAggregation(field string, size int) Aggregation {
	body := map[string]any{"field": field}
	if size > 0 {
		body["size"] = size
	}
	return Aggregation{"terms": body}
}

// DateHistogramAggregation creates a calendar interval date histogram aggregation.
func DateHistogramAggregation(field string, interval string) Aggregation {
	return Aggregation{"date_histogram": map[string]any{"field": field, "calendar_interval": interval}}
}

// RangeAggregation creates a numeric/date range aggregation.
func RangeAggregation(field string, ranges []map[string]any) Aggregation {
	return Aggregation{"range": map[string]any{"field": field, "ranges": ranges}}
}

// BuildSearchBody builds the Elasticsearch JSON body for a fuzzy search request.
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

// BuildAggregationBody builds the Elasticsearch JSON body for aggregation queries.
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

// FuzzySearch executes a fuzzy search and parses hits, highlights and aggregations.
func (s *Searcher) FuzzySearch(ctx context.Context, req FuzzySearchRequest) (*SearchResult, error) {
	body, err := BuildSearchBody(req)
	if err != nil {
		return nil, err
	}
	return s.doSearch(ctx, req.Index, body)
}

// Aggregate executes an aggregation query and parses aggregation buckets.
func (s *Searcher) Aggregate(ctx context.Context, req AggregationRequest) (*SearchResult, error) {
	body, err := BuildAggregationBody(req)
	if err != nil {
		return nil, err
	}
	return s.doSearch(ctx, req.Index, body)
}

// Search executes a raw Elasticsearch query body and parses the normalized response.
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
