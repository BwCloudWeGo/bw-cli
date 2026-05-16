package esx_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/esx"
)

func TestDefaultConfig(t *testing.T) {
	cfg := esx.DefaultConfig()

	require.Equal(t, []string{"http://127.0.0.1:9200"}, cfg.Addresses)
	require.Empty(t, cfg.Username)
	require.Empty(t, cfg.Password)
}

func TestNewClientOnlyRequiresAddresses(t *testing.T) {
	client, err := esx.NewClient(esx.Config{
		Addresses: []string{"http://127.0.0.1:9200"},
	})

	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestBuildFuzzySearchBodyIncludesHighlightAndFilters(t *testing.T) {
	body, err := esx.BuildSearchBody(esx.FuzzySearchRequest{
		Index:   "notes",
		Keyword: "golang",
		Fields:  []string{"title^2", "content"},
		From:    10,
		Size:    5,
		Filters: []esx.Filter{
			esx.TermFilter("status", "published"),
			esx.RangeFilter("created_at", map[string]any{"gte": "2026-01-01"}),
		},
		Sort: []esx.Sort{
			esx.SortField("created_at", "desc"),
		},
		Highlight: esx.HighlightConfig{
			Fields:            []string{"title", "content"},
			PreTags:           []string{"<mark>"},
			PostTags:          []string{"</mark>"},
			FragmentSize:      80,
			NumberOfFragments: 2,
		},
	})

	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"from": float64(10),
		"size": float64(5),
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{
					map[string]any{"term": map[string]any{"status": "published"}},
					map[string]any{"range": map[string]any{"created_at": map[string]any{"gte": "2026-01-01"}}},
				},
				"must": map[string]any{
					"multi_match": map[string]any{
						"fields":    []any{"title^2", "content"},
						"fuzziness": "AUTO",
						"operator":  "or",
						"query":     "golang",
						"type":      "best_fields",
					},
				},
			},
		},
		"highlight": map[string]any{
			"fields": map[string]any{
				"title":   map[string]any{},
				"content": map[string]any{},
			},
			"fragment_size":       float64(80),
			"number_of_fragments": float64(2),
			"post_tags":           []any{"</mark>"},
			"pre_tags":            []any{"<mark>"},
		},
		"sort": []any{
			map[string]any{"created_at": map[string]any{"order": "desc"}},
		},
	}, decodeJSONBody(t, body))
}

func TestBuildAggregationBodySupportsTermsAndDateHistogram(t *testing.T) {
	body, err := esx.BuildAggregationBody(esx.AggregationRequest{
		Index: "notes",
		Filters: []esx.Filter{
			esx.TermFilter("status", "published"),
		},
		Aggregations: map[string]esx.Aggregation{
			"by_author": esx.TermsAggregation("author_id", 10),
			"by_day":    esx.DateHistogramAggregation("created_at", "day"),
		},
	})

	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"size": float64(0),
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{
					map[string]any{"term": map[string]any{"status": "published"}},
				},
			},
		},
		"aggs": map[string]any{
			"by_author": map[string]any{
				"terms": map[string]any{"field": "author_id", "size": float64(10)},
			},
			"by_day": map[string]any{
				"date_histogram": map[string]any{"calendar_interval": "day", "field": "created_at"},
			},
		},
	}, decodeJSONBody(t, body))
}

func TestSearchParsesHitsHighlightsAndAggregations(t *testing.T) {
	searcher := esx.NewSearcher(&fakeSearchExecutor{
		response: `{
		  "hits": {
		    "total": {"value": 1},
		    "hits": [{
		      "_id": "note-1",
		      "_index": "notes",
		      "_score": 2.5,
		      "_source": {"title": "Golang search"},
		      "highlight": {"title": ["<em>Golang</em> search"]}
		    }]
		  },
		  "aggregations": {
		    "by_author": {"buckets": [{"key": "u1", "doc_count": 1}]}
		  }
		}`,
	})

	result, err := searcher.FuzzySearch(context.Background(), esx.FuzzySearchRequest{
		Index:   "notes",
		Keyword: "golang",
		Fields:  []string{"title"},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, result.Hits, 1)
	require.Equal(t, "note-1", result.Hits[0].ID)
	require.JSONEq(t, `{"title":"Golang search"}`, string(result.Hits[0].Source))
	require.Equal(t, []string{"<em>Golang</em> search"}, result.Hits[0].Highlight["title"])
	require.JSONEq(t, `{"by_author":{"buckets":[{"key":"u1","doc_count":1}]}}`, string(result.Aggregations))
}

func TestSearchExecutesRawDSL(t *testing.T) {
	executor := &fakeSearchExecutor{response: `{"hits":{"total":0,"hits":[]}}`}
	searcher := esx.NewSearcher(executor)

	result, err := searcher.Search(context.Background(), esx.SearchRequest{
		Index: "notes",
		Body: map[string]any{
			"query": map[string]any{
				"term": map[string]any{"status": "published"},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(0), result.Total)
	require.Equal(t, []string{"notes"}, executor.index)
	require.Equal(t, map[string]any{
		"query": map[string]any{
			"term": map[string]any{"status": "published"},
		},
	}, decodeJSONObject(t, executor.body))
}

func decodeJSONBody(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	return decodeJSONObject(t, data)
}

func decodeJSONObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

type fakeSearchExecutor struct {
	response string
	index    []string
	body     []byte
}

func (f *fakeSearchExecutor) Search(ctx context.Context, index []string, body io.Reader) (*esapi.Response, error) {
	data, _ := io.ReadAll(body)
	f.index = index
	f.body = data
	return &esapi.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(f.response)),
	}, nil
}
