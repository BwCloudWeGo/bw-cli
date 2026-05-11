package scaffold

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// RelationalSchemaMetadata is the inspected database shape used by templates.
type RelationalSchemaMetadata struct {
	Primary   TableMetadata
	Relations []RelationMetadata
}

// TableMetadata describes one relational table.
type TableMetadata struct {
	TableName  string
	Columns    []ColumnMetadata
	PrimaryKey ColumnMetadata
}

// ColumnMetadata describes one relational column and its generated code types.
type ColumnMetadata struct {
	ColumnName string
	DBType     string
	GoName     string
	GoType     string
	ProtoName  string
	ProtoType  string
	Nullable   bool
	PrimaryKey bool
}

// RelationMetadata describes an inspected associated table and checked fields.
type RelationMetadata struct {
	Schema        RelationSchema
	Table         TableMetadata
	LocalColumn   ColumnMetadata
	ForeignColumn ColumnMetadata
}

func (table TableMetadata) ColumnByName(name string) ColumnMetadata {
	for _, column := range table.Columns {
		if column.ColumnName == name {
			return column
		}
	}
	return ColumnMetadata{}
}

func inspectRelationalSchema(db *gorm.DB, driver string, schema ServiceSchema) (RelationalSchemaMetadata, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	primary, err := inspectTable(db, driver, schema.Table)
	if err != nil {
		return RelationalSchemaMetadata{}, err
	}
	if err := ensureSinglePrimaryKey(primary); err != nil {
		return RelationalSchemaMetadata{}, err
	}

	metadata := RelationalSchemaMetadata{Primary: primary, Relations: make([]RelationMetadata, 0, len(schema.Relations))}
	for _, relation := range schema.Relations {
		table, err := inspectTable(db, driver, relation.Table)
		if err != nil {
			return RelationalSchemaMetadata{}, err
		}
		localColumn, ok := findColumn(primary, relation.LocalField)
		if !ok {
			return RelationalSchemaMetadata{}, fmt.Errorf("relation %q: local_field %q not found in table %q", relation.Name, relation.LocalField, primary.TableName)
		}
		foreignColumn, ok := findColumn(table, relation.ForeignField)
		if !ok {
			return RelationalSchemaMetadata{}, fmt.Errorf("relation %q: foreign_field %q not found in table %q", relation.Name, relation.ForeignField, table.TableName)
		}
		metadata.Relations = append(metadata.Relations, RelationMetadata{
			Schema:        relation,
			Table:         table,
			LocalColumn:   localColumn,
			ForeignColumn: foreignColumn,
		})
	}
	return metadata, nil
}

func inspectTable(db *gorm.DB, driver string, table string) (TableMetadata, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return TableMetadata{}, fmt.Errorf("table is required")
	}
	var (
		columns []ColumnMetadata
		err     error
	)
	switch driver {
	case "sqlite":
		columns, err = inspectSQLiteColumns(db, table)
	case "mysql":
		columns, err = inspectMySQLColumns(db, table)
	case "postgres", "postgresql":
		columns, err = inspectPostgreSQLColumns(db, table)
	default:
		return TableMetadata{}, fmt.Errorf("unsupported database driver %q", driver)
	}
	if err != nil {
		return TableMetadata{}, err
	}
	if len(columns) == 0 {
		return TableMetadata{}, fmt.Errorf("table %q not found in configured %s database", table, driver)
	}
	result := TableMetadata{TableName: table, Columns: columns}
	for _, column := range columns {
		if column.PrimaryKey {
			result.PrimaryKey = column
			break
		}
	}
	return result, nil
}

type sqliteColumnInfo struct {
	CID       int
	Name      string
	Type      string
	NotNull   int
	DfltValue sql.NullString
	PK        int
}

func inspectSQLiteColumns(db *gorm.DB, table string) ([]ColumnMetadata, error) {
	var rows []sqliteColumnInfo
	if err := db.Raw(fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdent(table))).Scan(&rows).Error; err != nil {
		return nil, err
	}
	columns := make([]ColumnMetadata, 0, len(rows))
	for _, row := range rows {
		columns = append(columns, newColumnMetadata(row.Name, row.Type, row.NotNull == 0, row.PK > 0))
	}
	return columns, nil
}

type informationSchemaColumn struct {
	ColumnName string
	DataType   string
	IsNullable string
	ColumnKey  string
}

func inspectMySQLColumns(db *gorm.DB, table string) ([]ColumnMetadata, error) {
	var rows []informationSchemaColumn
	err := db.Raw(`
SELECT column_name, data_type, is_nullable, column_key
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = ?
ORDER BY ordinal_position`, table).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return informationSchemaColumnsToMetadata(rows, "PRI"), nil
}

func inspectPostgreSQLColumns(db *gorm.DB, table string) ([]ColumnMetadata, error) {
	var rows []informationSchemaColumn
	err := db.Raw(`
SELECT c.column_name, c.data_type, c.is_nullable,
       CASE WHEN tc.constraint_type = 'PRIMARY KEY' THEN 'PRI' ELSE '' END AS column_key
FROM information_schema.columns c
LEFT JOIN information_schema.key_column_usage kcu
  ON c.table_schema = kcu.table_schema
 AND c.table_name = kcu.table_name
 AND c.column_name = kcu.column_name
LEFT JOIN information_schema.table_constraints tc
  ON kcu.constraint_schema = tc.constraint_schema
 AND kcu.constraint_name = tc.constraint_name
 AND tc.constraint_type = 'PRIMARY KEY'
WHERE c.table_schema = current_schema() AND c.table_name = ?
ORDER BY c.ordinal_position`, table).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return informationSchemaColumnsToMetadata(rows, "PRI"), nil
}

func informationSchemaColumnsToMetadata(rows []informationSchemaColumn, primaryKeyValue string) []ColumnMetadata {
	columns := make([]ColumnMetadata, 0, len(rows))
	for _, row := range rows {
		columns = append(columns, newColumnMetadata(row.ColumnName, row.DataType, strings.EqualFold(row.IsNullable, "YES"), row.ColumnKey == primaryKeyValue))
	}
	return columns
}

func newColumnMetadata(name string, dbType string, nullable bool, primaryKey bool) ColumnMetadata {
	goType, protoType := mapDatabaseType(dbType)
	return ColumnMetadata{
		ColumnName: strings.TrimSpace(name),
		DBType:     strings.TrimSpace(dbType),
		GoName:     toGoFieldName(name),
		GoType:     goType,
		ProtoName:  strings.TrimSpace(name),
		ProtoType:  protoType,
		Nullable:   nullable,
		PrimaryKey: primaryKey,
	}
}

func mapDatabaseType(dbType string) (string, string) {
	normalized := normalizeDatabaseType(dbType)
	switch normalized {
	case "int", "integer", "smallint", "mediumint":
		return "int32", "int32"
	case "bigint":
		return "int64", "int64"
	case "bool", "boolean", "tinyint(1)":
		return "bool", "bool"
	case "decimal", "numeric":
		return "string", "string"
	case "float", "double", "real":
		return "float64", "double"
	case "date", "datetime", "timestamp", "time":
		return "time.Time", "string"
	case "blob", "binary", "varbinary", "bytea", "bytes":
		return "[]byte", "bytes"
	default:
		return "string", "string"
	}
}

var dbTypeArgsPattern = regexp.MustCompile(`\s+`)

func normalizeDatabaseType(dbType string) string {
	value := strings.ToLower(strings.TrimSpace(dbType))
	value = dbTypeArgsPattern.ReplaceAllString(value, " ")
	if strings.HasPrefix(value, "tinyint(1)") {
		return "tinyint(1)"
	}
	if index := strings.Index(value, "("); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func ensureSinglePrimaryKey(table TableMetadata) error {
	count := 0
	for _, column := range table.Columns {
		if column.PrimaryKey {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("primary table %q must have exactly one primary key", table.TableName)
	}
	return nil
}

func findColumn(table TableMetadata, name string) (ColumnMetadata, bool) {
	for _, column := range table.Columns {
		if column.ColumnName == name {
			return column, true
		}
	}
	return ColumnMetadata{}, false
}

func quoteSQLiteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

var commonInitialisms = map[string]string{
	"api":  "API",
	"http": "HTTP",
	"id":   "ID",
	"ip":   "IP",
	"url":  "URL",
	"uuid": "UUID",
}

func toGoFieldName(columnName string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(columnName), func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var b strings.Builder
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		if initialism, ok := commonInitialisms[part]; ok {
			b.WriteString(initialism)
			continue
		}
		b.WriteString(toPascal([]string{part}))
	}
	return b.String()
}
