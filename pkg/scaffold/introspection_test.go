package scaffold

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInspectRelationalSchemaReadsSQLiteTable(t *testing.T) {
	db := openSQLiteSchema(t, `
CREATE TABLE orders (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	total_amount DECIMAL(10,2) NOT NULL,
	paid BOOLEAN NOT NULL DEFAULT false,
	created_at DATETIME
);`)

	metadata, err := inspectRelationalSchema(db, "sqlite", ServiceSchema{Table: "orders"})

	require.NoError(t, err)
	require.Equal(t, "orders", metadata.Primary.TableName)
	require.Equal(t, "id", metadata.Primary.PrimaryKey.ColumnName)
	require.Len(t, metadata.Primary.Columns, 5)
	require.Equal(t, "ID", metadata.Primary.Columns[0].GoName)
	require.Equal(t, "string", metadata.Primary.Columns[0].GoType)
	require.Equal(t, "string", metadata.Primary.ColumnByName("total_amount").ProtoType)
	require.Equal(t, "bool", metadata.Primary.ColumnByName("paid").GoType)
	require.Equal(t, "time.Time", metadata.Primary.ColumnByName("created_at").GoType)
}

func TestInspectRelationalSchemaRejectsMissingTable(t *testing.T) {
	db := openSQLiteSchema(t, `CREATE TABLE users (id TEXT PRIMARY KEY);`)

	_, err := inspectRelationalSchema(db, "sqlite", ServiceSchema{Table: "orders"})

	require.Error(t, err)
	require.Contains(t, err.Error(), `table "orders" not found`)
}

func TestInspectRelationalSchemaValidatesRelationFields(t *testing.T) {
	db := openSQLiteSchema(t, `
CREATE TABLE orders (id TEXT PRIMARY KEY);
CREATE TABLE order_items (id TEXT PRIMARY KEY, order_id TEXT NOT NULL);
`)

	metadata, err := inspectRelationalSchema(db, "sqlite", ServiceSchema{
		Table: "orders",
		Relations: []RelationSchema{{
			Name:         "order_items",
			Table:        "order_items",
			Type:         relationHasMany,
			LocalField:   "id",
			ForeignField: "order_id",
		}},
	})

	require.NoError(t, err)
	require.Len(t, metadata.Relations, 1)
	require.Equal(t, "order_items", metadata.Relations[0].Table.TableName)
	require.Equal(t, "id", metadata.Relations[0].LocalColumn.ColumnName)
	require.Equal(t, "order_id", metadata.Relations[0].ForeignColumn.ColumnName)
}

func TestInspectRelationalSchemaRejectsMissingRelationField(t *testing.T) {
	db := openSQLiteSchema(t, `
CREATE TABLE orders (id TEXT PRIMARY KEY);
CREATE TABLE order_items (id TEXT PRIMARY KEY);
`)

	_, err := inspectRelationalSchema(db, "sqlite", ServiceSchema{
		Table: "orders",
		Relations: []RelationSchema{{
			Name:         "order_items",
			Table:        "order_items",
			Type:         relationHasMany,
			LocalField:   "id",
			ForeignField: "order_id",
		}},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), `foreign_field "order_id" not found`)
}

func openSQLiteSchema(t *testing.T, schemaSQL string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(schemaSQL).Error)
	return db
}
