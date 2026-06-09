package scaffold

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInspectSQLiteDatabaseSchemaDetectsTablesColumnsAndForeignKeys(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "designer.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`create table product_spu (id text primary key, name text not null)`).Error)
	require.NoError(t, db.Exec(`create table product_sku (id text primary key, spu_id text not null, price integer, foreign key(spu_id) references product_spu(id))`).Error)

	snapshot, err := InspectDatabaseSchema(DatabaseConnection{
		Driver: "sqlite",
		DSN:    dbPath,
	}, "")

	require.NoError(t, err)
	require.Len(t, snapshot.Tables, 2)
	require.Equal(t, "product_spu", snapshot.Tables[0].Name)
	require.Equal(t, "id", snapshot.Tables[0].Columns[0].Name)
	require.True(t, snapshot.Tables[0].Columns[0].PrimaryKey)
	require.Len(t, snapshot.ForeignKeys, 1)
	require.Equal(t, "product_sku", snapshot.ForeignKeys[0].FromTable)
	require.Equal(t, "spu_id", snapshot.ForeignKeys[0].FromColumn)
	require.Equal(t, "product_spu", snapshot.ForeignKeys[0].ToTable)
	require.Equal(t, "id", snapshot.ForeignKeys[0].ToColumn)
}
