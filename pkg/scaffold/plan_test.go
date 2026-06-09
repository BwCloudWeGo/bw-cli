package scaffold

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateGenerationPlanSupportsSingleAndMultiTable(t *testing.T) {
	single := GenerationPlan{
		ServiceName: "product",
		RootTable:   "product_spu",
		Tables:      []string{"product_spu"},
	}
	require.NoError(t, ValidateGenerationPlan(single))

	multi := GenerationPlan{
		ServiceName: "product",
		RootTable:   "product_spu",
		Tables:      []string{"product_spu", "product_sku"},
		Relationships: []TableRelationship{{
			Type:       RelationshipOneToMany,
			FromTable:  "product_sku",
			FromColumn: "spu_id",
			ToTable:    "product_spu",
			ToColumn:   "id",
		}},
	}
	require.NoError(t, ValidateGenerationPlan(multi))
}

func TestSaveAndLoadGenerationPlanRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "product.json")
	plan := GenerationPlan{
		ServiceName: "product",
		RootTable:   "product_spu",
		Tables:      []string{"product_spu"},
	}

	require.NoError(t, SaveGenerationPlan(path, plan))
	loaded, err := LoadGenerationPlan(path)

	require.NoError(t, err)
	require.Equal(t, plan.ServiceName, loaded.ServiceName)
	require.Equal(t, plan.RootTable, loaded.RootTable)
	require.Equal(t, plan.Tables, loaded.Tables)
}
