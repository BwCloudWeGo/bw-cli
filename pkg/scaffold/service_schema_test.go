package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadServiceSchemaParsesRelations(t *testing.T) {
	path := writeSchemaFile(t, `table: orders
resource: order
exclude_fields:
  - password
readonly_fields:
  - id
relations:
  - name: order_items
    table: order_items
    type: has_many
    local_field: id
    foreign_field: order_id
    methods:
      - ListOrderItemsByOrderID
`)

	schema, err := loadServiceSchema(path)

	require.NoError(t, err)
	require.Equal(t, "orders", schema.Table)
	require.Equal(t, "order", schema.Resource)
	require.Equal(t, []string{"password"}, schema.ExcludeFields)
	require.Equal(t, []string{"id"}, schema.ReadonlyFields)
	require.Len(t, schema.Relations, 1)
	require.Equal(t, "order_items", schema.Relations[0].Name)
	require.Equal(t, "has_many", schema.Relations[0].Type)
	require.Equal(t, []string{"ListOrderItemsByOrderID"}, schema.Relations[0].Methods)
}

func TestMergeServiceSchemaUsesCommandTable(t *testing.T) {
	schema := ServiceSchema{Table: "orders_from_yaml"}

	merged, err := mergeServiceSchema(schema, "orders_from_flag", "order")

	require.NoError(t, err)
	require.Equal(t, "orders_from_flag", merged.Table)
	require.Equal(t, "order", merged.Resource)
}

func TestValidateServiceSchemaRejectsUnsupportedRelationType(t *testing.T) {
	schema := ServiceSchema{
		Table: "orders",
		Relations: []RelationSchema{{
			Name:         "items",
			Table:        "order_items",
			Type:         "many_to_many",
			LocalField:   "id",
			ForeignField: "order_id",
		}},
	}

	_, err := mergeServiceSchema(schema, "", "order")

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported relation type")
}

func TestValidateServiceSchemaRejectsMissingRelationFields(t *testing.T) {
	schema := ServiceSchema{
		Table: "orders",
		Relations: []RelationSchema{{
			Name:  "items",
			Table: "order_items",
			Type:  "has_many",
		}},
	}

	_, err := mergeServiceSchema(schema, "", "order")

	require.Error(t, err)
	require.Contains(t, err.Error(), "local_field")
}

func writeSchemaFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "service.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
