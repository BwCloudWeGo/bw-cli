package scaffold

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DatabaseConnection 描述 designer 读取数据库结构时使用的连接信息。
type DatabaseConnection struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

// SchemaSnapshot 是可视化界面展示和生成计划引用的数据库结构快照。
type SchemaSnapshot struct {
	Driver      string             `json:"driver"`
	Schema      string             `json:"schema,omitempty"`
	Tables      []TableSchema      `json:"tables"`
	ForeignKeys []ForeignKeySchema `json:"foreign_keys"`
}

// TableSchema 描述一张表及其字段。
type TableSchema struct {
	Name    string         `json:"name"`
	Columns []ColumnSchema `json:"columns"`
}

// ColumnSchema 描述数据库字段。
type ColumnSchema struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Nullable      bool   `json:"nullable"`
	PrimaryKey    bool   `json:"primary_key"`
	AutoIncrement bool   `json:"auto_increment"`
	DefaultValue  string `json:"default_value,omitempty"`
}

// ForeignKeySchema 描述数据库中已经存在的外键关系。
type ForeignKeySchema struct {
	FromTable  string `json:"from_table"`
	FromColumn string `json:"from_column"`
	ToTable    string `json:"to_table"`
	ToColumn   string `json:"to_column"`
}

// InspectDatabaseSchema 连接数据库并读取表、字段和外键。
func InspectDatabaseSchema(conn DatabaseConnection, schema string) (SchemaSnapshot, error) {
	driver := strings.ToLower(strings.TrimSpace(conn.Driver))
	db, err := openDesignerDB(driver, strings.TrimSpace(conn.DSN))
	if err != nil {
		return SchemaSnapshot{}, err
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}
	switch driver {
	case "sqlite":
		return inspectSQLiteSchema(db, driver)
	case "mysql":
		return inspectInformationSchema(db, driver, schema, true)
	case "postgres", "postgresql", "pg":
		return inspectInformationSchema(db, driver, schema, false)
	default:
		return SchemaSnapshot{}, fmt.Errorf("designer does not support database driver %q", conn.Driver)
	}
}

func openDesignerDB(driver string, dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("%s dsn is required", driver)
	}
	cfg := &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)}
	switch driver {
	case "sqlite":
		if !filepath.IsAbs(dsn) && dsn != ":memory:" {
			abs, err := filepath.Abs(dsn)
			if err != nil {
				return nil, err
			}
			dsn = abs
		}
		return gorm.Open(sqlite.Open(dsn), cfg)
	case "mysql":
		return gorm.Open(mysql.Open(dsn), cfg)
	case "postgres", "postgresql", "pg":
		return gorm.Open(postgres.Open(dsn), cfg)
	default:
		return nil, fmt.Errorf("designer does not support database driver %q", driver)
	}
}

func inspectSQLiteSchema(db *gorm.DB, driver string) (SchemaSnapshot, error) {
	tableNames := []string{}
	if err := db.Raw(`select name from sqlite_master where type = 'table' and name not like 'sqlite_%' order by rowid`).Scan(&tableNames).Error; err != nil {
		return SchemaSnapshot{}, err
	}
	tables := make([]TableSchema, 0, len(tableNames))
	foreignKeys := []ForeignKeySchema{}
	for _, table := range tableNames {
		columns, err := inspectSQLiteTableColumns(db, table)
		if err != nil {
			return SchemaSnapshot{}, err
		}
		tables = append(tables, TableSchema{Name: table, Columns: columns})
		fks, err := inspectSQLiteForeignKeys(db, table)
		if err != nil {
			return SchemaSnapshot{}, err
		}
		foreignKeys = append(foreignKeys, fks...)
	}
	return SchemaSnapshot{Driver: driver, Tables: tables, ForeignKeys: foreignKeys}, nil
}

func inspectSQLiteTableColumns(db *gorm.DB, table string) ([]ColumnSchema, error) {
	rows, err := db.Raw("PRAGMA table_info(" + quoteSQLiteIdent(table) + ")").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []ColumnSchema{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		column := ColumnSchema{
			Name:       name,
			Type:       typ,
			Nullable:   notNull == 0,
			PrimaryKey: pk > 0,
		}
		if defaultValue.Valid {
			column.DefaultValue = defaultValue.String
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func inspectSQLiteForeignKeys(db *gorm.DB, table string) ([]ForeignKeySchema, error) {
	rows, err := db.Raw("PRAGMA foreign_key_list(" + quoteSQLiteIdent(table) + ")").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	foreignKeys := []ForeignKeySchema{}
	for rows.Next() {
		var id, seq int
		var toTable, fromColumn, toColumn, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &toTable, &fromColumn, &toColumn, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, ForeignKeySchema{
			FromTable:  table,
			FromColumn: fromColumn,
			ToTable:    toTable,
			ToColumn:   toColumn,
		})
	}
	return foreignKeys, rows.Err()
}

func inspectInformationSchema(db *gorm.DB, driver string, schema string, mysqlDriver bool) (SchemaSnapshot, error) {
	schema = strings.TrimSpace(schema)
	if mysqlDriver && schema == "" {
		if err := db.Raw("select database()").Scan(&schema).Error; err != nil {
			return SchemaSnapshot{}, err
		}
	}
	if !mysqlDriver && schema == "" {
		schema = "public"
	}

	query := `select table_name, column_name, data_type, is_nullable,
case when column_key = 'PRI' then 'PRI' else '' end as column_key,
extra,
column_default
from information_schema.columns
where table_schema = ?
order by table_name, ordinal_position`
	if !mysqlDriver {
		query = `select c.table_name, c.column_name, c.data_type, c.is_nullable,
case when tc.constraint_type = 'PRIMARY KEY' then 'PRI' else '' end as column_key,
'' as extra,
c.column_default
from information_schema.columns c
left join information_schema.key_column_usage kcu
  on c.table_schema = kcu.table_schema and c.table_name = kcu.table_name and c.column_name = kcu.column_name
left join information_schema.table_constraints tc
  on kcu.constraint_schema = tc.constraint_schema and kcu.constraint_name = tc.constraint_name
where c.table_schema = ?
order by c.table_name, c.ordinal_position`
	}
	rows, err := db.Raw(query, schema).Rows()
	if err != nil {
		return SchemaSnapshot{}, err
	}
	defer rows.Close()

	tableMap := map[string][]ColumnSchema{}
	tableOrder := []string{}
	for rows.Next() {
		var tableName, columnName, typ, nullable, key, extra string
		var defaultValue sql.NullString
		if err := rows.Scan(&tableName, &columnName, &typ, &nullable, &key, &extra, &defaultValue); err != nil {
			return SchemaSnapshot{}, err
		}
		if _, ok := tableMap[tableName]; !ok {
			tableOrder = append(tableOrder, tableName)
		}
		column := ColumnSchema{
			Name:          columnName,
			Type:          typ,
			Nullable:      strings.EqualFold(nullable, "YES"),
			PrimaryKey:    key == "PRI",
			AutoIncrement: strings.Contains(strings.ToLower(extra), "auto_increment"),
		}
		if defaultValue.Valid {
			column.DefaultValue = defaultValue.String
		}
		tableMap[tableName] = append(tableMap[tableName], column)
	}
	if err := rows.Err(); err != nil {
		return SchemaSnapshot{}, err
	}
	tables := make([]TableSchema, 0, len(tableOrder))
	for _, name := range tableOrder {
		tables = append(tables, TableSchema{Name: name, Columns: tableMap[name]})
	}
	foreignKeys, err := inspectInformationSchemaForeignKeys(db, schema, mysqlDriver)
	if err != nil {
		return SchemaSnapshot{}, err
	}
	return SchemaSnapshot{Driver: driver, Schema: schema, Tables: tables, ForeignKeys: foreignKeys}, nil
}

func inspectInformationSchemaForeignKeys(db *gorm.DB, schema string, mysqlDriver bool) ([]ForeignKeySchema, error) {
	query := `select table_name, column_name, referenced_table_name, referenced_column_name
from information_schema.key_column_usage
where table_schema = ? and referenced_table_name is not null
order by table_name, column_name`
	if !mysqlDriver {
		query = `select kcu.table_name, kcu.column_name, ccu.table_name, ccu.column_name
from information_schema.table_constraints tc
join information_schema.key_column_usage kcu
  on tc.constraint_schema = kcu.constraint_schema and tc.constraint_name = kcu.constraint_name
join information_schema.constraint_column_usage ccu
  on ccu.constraint_schema = tc.constraint_schema and ccu.constraint_name = tc.constraint_name
where tc.table_schema = ? and tc.constraint_type = 'FOREIGN KEY'
order by kcu.table_name, kcu.column_name`
	}
	rows, err := db.Raw(query, schema).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	foreignKeys := []ForeignKeySchema{}
	for rows.Next() {
		var fromTable, fromColumn, toTable, toColumn string
		if err := rows.Scan(&fromTable, &fromColumn, &toTable, &toColumn); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, ForeignKeySchema{
			FromTable:  fromTable,
			FromColumn: fromColumn,
			ToTable:    toTable,
			ToColumn:   toColumn,
		})
	}
	return foreignKeys, rows.Err()
}
