package builder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

type ColumnType interface {
	renderColumnType(dialect.Name) (string, error)
}

type portableColumnType struct {
	name      string
	length    int
	precision int
	scale     int
	keyString bool
}

func TextType() ColumnType      { return portableColumnType{name: "TEXT"} }
func BigIntType() ColumnType    { return portableColumnType{name: "BIGINT"} }
func IntegerType() ColumnType   { return portableColumnType{name: "INTEGER"} }
func SmallIntType() ColumnType  { return portableColumnType{name: "SMALLINT"} }
func BooleanType() ColumnType   { return portableColumnType{name: "BOOLEAN"} }
func RealType() ColumnType      { return portableColumnType{name: "REAL"} }
func DoubleType() ColumnType    { return portableColumnType{name: "DOUBLE"} }
func DateType() ColumnType      { return portableColumnType{name: "DATE"} }
func TimeType() ColumnType      { return portableColumnType{name: "TIME"} }
func TimestampType() ColumnType { return portableColumnType{name: "TIMESTAMP"} }
func JSONType() ColumnType      { return portableColumnType{name: "JSON"} }
func UUIDType() ColumnType      { return portableColumnType{name: "UUID"} }
func BinaryType() ColumnType    { return portableColumnType{name: "BINARY"} }

func DecimalType(precision, scale int) ColumnType {
	return portableColumnType{name: "DECIMAL", precision: precision, scale: scale}
}

func VarcharType(length int) ColumnType {
	return portableColumnType{name: "VARCHAR", length: length}
}

// TextKeyType is an index-safe text identity. MySQL requires a bounded
// VARCHAR; PostgreSQL and SQLite use TEXT without leaking that distinction to
// schema owners.
func TextKeyType(maxLength int) ColumnType {
	return portableColumnType{name: "TEXT", length: maxLength, keyString: true}
}

func (t portableColumnType) renderColumnType(name dialect.Name) (string, error) {
	if t.keyString {
		if t.length < 1 || t.length > 65535 {
			return "", fmt.Errorf("SQL text key length must be between 1 and 65535")
		}
		if name == dialect.MySQL {
			return "VARCHAR(" + strconv.Itoa(t.length) + ")", nil
		}
		if name == dialect.Postgres || name == dialect.SQLite {
			return "TEXT", nil
		}
	}
	if t.name == "VARCHAR" {
		if t.length < 1 || t.length > 65535 {
			return "", fmt.Errorf("SQL varchar length must be between 1 and 65535")
		}
		return "VARCHAR(" + strconv.Itoa(t.length) + ")", nil
	}
	if t.name == "DECIMAL" {
		if t.precision < 1 || t.precision > 38 || t.scale < 0 || t.scale > t.precision {
			return "", fmt.Errorf("SQL decimal precision must be between 1 and 38 and scale between 0 and precision")
		}
		return "DECIMAL(" + strconv.Itoa(t.precision) + "," + strconv.Itoa(t.scale) + ")", nil
	}
	switch t.name {
	case "TEXT", "BIGINT", "INTEGER", "SMALLINT", "BOOLEAN", "REAL", "DATE", "TIME", "TIMESTAMP":
		return t.name, nil
	case "DOUBLE":
		if name == dialect.Postgres {
			return "DOUBLE PRECISION", nil
		}
		return "DOUBLE", nil
	case "JSON":
		switch name {
		case dialect.Postgres:
			return "JSONB", nil
		case dialect.MySQL:
			return "JSON", nil
		case dialect.SQLite:
			return "TEXT", nil
		}
	case "UUID":
		if name == dialect.Postgres {
			return "UUID", nil
		}
		return "VARCHAR(36)", nil
	case "BINARY":
		switch name {
		case dialect.Postgres:
			return "BYTEA", nil
		case dialect.MySQL:
			return "LONGBLOB", nil
		case dialect.SQLite:
			return "BLOB", nil
		}
	default:
		return "", fmt.Errorf("unsupported SQL column type %q", t.name)
	}
	return "", fmt.Errorf("unsupported SQL column type %q for dialect %q", t.name, name)
}

type SchemaColumn struct {
	name         string
	typeOf       ColumnType
	notNull      bool
	defaultValue string
}

func DefineColumn(name string, typeOf ColumnType) SchemaColumn {
	return SchemaColumn{name: strings.TrimSpace(name), typeOf: typeOf}
}

func (c SchemaColumn) NotNull() SchemaColumn {
	c.notNull = true
	return c
}

// Default sets a portable, trusted SQL default. The accepted values are kept
// deliberately closed so schema construction cannot become a raw-SQL escape.
func (c SchemaColumn) Default(value string) SchemaColumn {
	c.defaultValue = strings.TrimSpace(value)
	return c
}

var systemColumns = []SchemaColumn{
	DefineColumn("workspace_id", TextKeyType(255)).NotNull(),
	DefineColumn("id", TextKeyType(255)).NotNull(),
	DefineColumn("created_at", TimestampType()).NotNull().Default("current_timestamp"),
	DefineColumn("updated_at", TimestampType()).NotNull().Default("current_timestamp"),
	DefineColumn("deleted", BooleanType()).NotNull().Default("false"),
	DefineColumn("ext_info", JSONType()).NotNull().Default("empty_json"),
	DefineColumn("create_user_id", TextKeyType(255)),
	DefineColumn("update_user_id", TextKeyType(255)),
}

type tableConstraint struct {
	kind    string
	columns []string
}

type CreateTableBuilder struct {
	renderer             Renderer
	table                string
	ifNotExists          bool
	withoutSystemColumns bool
	columns              []SchemaColumn
	constraints          []tableConstraint
}

func NewCreateTableBuilder(renderer Renderer, table string) *CreateTableBuilder {
	return &CreateTableBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}

func (b *CreateTableBuilder) IfNotExists() *CreateTableBuilder {
	b.ifNotExists = true
	return b
}

// WithoutSystemColumns declares an infrastructure-owned physical table. Such
// tables keep their explicit schema and do not receive Domainry Record system
// columns. Business Record tables should use the default behavior.
func (b *CreateTableBuilder) WithoutSystemColumns() *CreateTableBuilder {
	b.withoutSystemColumns = true
	return b
}

func (b *CreateTableBuilder) Columns(columns ...SchemaColumn) *CreateTableBuilder {
	b.columns = append([]SchemaColumn(nil), columns...)
	return b
}

func (b *CreateTableBuilder) PrimaryKey(columns ...string) *CreateTableBuilder {
	b.constraints = append(b.constraints, tableConstraint{kind: "PRIMARY KEY", columns: append([]string(nil), columns...)})
	return b
}

func (b *CreateTableBuilder) Unique(columns ...string) *CreateTableBuilder {
	b.constraints = append(b.constraints, tableConstraint{kind: "UNIQUE", columns: append([]string(nil), columns...)})
	return b
}

func (b *CreateTableBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" {
		return "", nil, fmt.Errorf("SQL create table requires renderer and table")
	}
	name, ok := b.renderer.(namedRenderer)
	if !ok {
		return "", nil, fmt.Errorf("SQL create table requires a named dialect renderer")
	}
	columns := append([]SchemaColumn(nil), b.columns...)
	if !b.withoutSystemColumns {
		var err error
		columns, err = mergeSystemColumns(columns)
		if err != nil {
			return "", nil, err
		}
	} else if len(columns) == 0 {
		return "", nil, fmt.Errorf("SQL infrastructure table requires columns")
	}
	defined := make(map[string]struct{}, len(columns))
	parts := make([]string, 0, len(columns)+len(b.constraints))
	for _, column := range columns {
		if column.name == "" || column.typeOf == nil {
			return "", nil, fmt.Errorf("SQL table column requires name and type")
		}
		key := strings.ToLower(column.name)
		if _, exists := defined[key]; exists {
			return "", nil, fmt.Errorf("SQL table column %q is duplicated", column.name)
		}
		defined[key] = struct{}{}
		columnType, err := column.typeOf.renderColumnType(name.Name())
		if err != nil {
			return "", nil, err
		}
		definition := b.renderer.Identifier(column.name) + " " + columnType
		if column.notNull {
			definition += " NOT NULL"
		}
		if column.defaultValue != "" {
			value, err := renderColumnDefault(name.Name(), column.defaultValue)
			if err != nil {
				return "", nil, fmt.Errorf("SQL table column %q: %w", column.name, err)
			}
			definition += " DEFAULT " + value
		}
		parts = append(parts, definition)
	}
	for _, constraint := range b.constraints {
		if len(constraint.columns) == 0 {
			return "", nil, fmt.Errorf("SQL %s requires columns", strings.ToLower(constraint.kind))
		}
		columns := make([]string, len(constraint.columns))
		for index, column := range constraint.columns {
			if _, exists := defined[strings.ToLower(strings.TrimSpace(column))]; !exists {
				return "", nil, fmt.Errorf("SQL %s references undefined column %q", strings.ToLower(constraint.kind), column)
			}
			columns[index] = b.renderer.Identifier(column)
		}
		parts = append(parts, constraint.kind+" ("+strings.Join(columns, ", ")+")")
	}
	statement := "CREATE TABLE "
	if b.ifNotExists {
		statement += "IF NOT EXISTS "
	}
	statement += b.renderer.Table(b.table) + " (" + strings.Join(parts, ", ") + ")"
	return statement, []any{}, nil
}

func mergeSystemColumns(columns []SchemaColumn) ([]SchemaColumn, error) {
	result := append([]SchemaColumn(nil), columns...)
	existing := make(map[string]SchemaColumn, len(columns))
	for _, column := range columns {
		key := strings.ToLower(column.name)
		if _, found := existing[key]; found {
			return nil, fmt.Errorf("SQL table column %q is duplicated", column.name)
		}
		existing[key] = column
	}
	for _, required := range systemColumns {
		if _, found := existing[required.name]; !found {
			result = append(result, required)
		}
	}
	return result, nil
}

func renderColumnDefault(name dialect.Name, value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "current_timestamp":
		return "CURRENT_TIMESTAMP", nil
	case "false":
		if name == dialect.SQLite {
			return "0", nil
		}
		return "FALSE", nil
	case "true":
		if name == dialect.SQLite {
			return "1", nil
		}
		return "TRUE", nil
	case "empty_json":
		switch name {
		case dialect.Postgres:
			return "'{}'::jsonb", nil
		case dialect.MySQL:
			return "(JSON_OBJECT())", nil
		case dialect.SQLite:
			return "'{}'", nil
		}
	}
	return "", fmt.Errorf("unsupported SQL column default %q", value)
}

type CreateIndexBuilder struct {
	renderer    Renderer
	index       string
	table       string
	columns     []string
	unique      bool
	ifNotExists bool
}

func NewCreateIndexBuilder(renderer Renderer, index, table string) *CreateIndexBuilder {
	return &CreateIndexBuilder{renderer: renderer, index: strings.TrimSpace(index), table: strings.TrimSpace(table)}
}

func (b *CreateIndexBuilder) Columns(columns ...string) *CreateIndexBuilder {
	b.columns = append([]string(nil), columns...)
	return b
}

func (b *CreateIndexBuilder) Unique() *CreateIndexBuilder {
	b.unique = true
	return b
}

func (b *CreateIndexBuilder) IfNotExists() *CreateIndexBuilder {
	b.ifNotExists = true
	return b
}

func (b *CreateIndexBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.index == "" || b.table == "" || len(b.columns) == 0 {
		return "", nil, fmt.Errorf("SQL create index requires renderer, index, table, and columns")
	}
	if b.ifNotExists {
		if named, ok := b.renderer.(namedRenderer); !ok || named.Name() == dialect.MySQL {
			return "", nil, fmt.Errorf("SQL create index IF NOT EXISTS is not supported by the active dialect")
		}
	}
	columns := make([]string, len(b.columns))
	for index, column := range b.columns {
		columns[index] = b.renderer.Identifier(column)
	}
	statement := "CREATE "
	if b.unique {
		statement += "UNIQUE "
	}
	statement += "INDEX "
	if b.ifNotExists {
		statement += "IF NOT EXISTS "
	}
	statement += b.renderer.Identifier(b.index) + " ON " + b.renderer.Table(b.table) + " (" + strings.Join(columns, ", ") + ")"
	return statement, []any{}, nil
}
