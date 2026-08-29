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
	keyString bool
}

func TextType() ColumnType    { return portableColumnType{name: "TEXT"} }
func BigIntType() ColumnType  { return portableColumnType{name: "BIGINT"} }
func IntegerType() ColumnType { return portableColumnType{name: "INTEGER"} }
func BooleanType() ColumnType { return portableColumnType{name: "BOOLEAN"} }

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
	switch t.name {
	case "TEXT", "BIGINT", "INTEGER", "BOOLEAN":
		return t.name, nil
	default:
		return "", fmt.Errorf("unsupported SQL column type %q", t.name)
	}
}

type SchemaColumn struct {
	name    string
	typeOf  ColumnType
	notNull bool
}

func DefineColumn(name string, typeOf ColumnType) SchemaColumn {
	return SchemaColumn{name: strings.TrimSpace(name), typeOf: typeOf}
}

func (c SchemaColumn) NotNull() SchemaColumn {
	c.notNull = true
	return c
}

type tableConstraint struct {
	kind    string
	columns []string
}

type CreateTableBuilder struct {
	renderer    Renderer
	table       string
	ifNotExists bool
	columns     []SchemaColumn
	constraints []tableConstraint
}

func NewCreateTableBuilder(renderer Renderer, table string) *CreateTableBuilder {
	return &CreateTableBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}

func (b *CreateTableBuilder) IfNotExists() *CreateTableBuilder {
	b.ifNotExists = true
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
	if b == nil || b.renderer == nil || b.table == "" || len(b.columns) == 0 {
		return "", nil, fmt.Errorf("SQL create table requires renderer, table, and columns")
	}
	name, ok := b.renderer.(namedRenderer)
	if !ok {
		return "", nil, fmt.Errorf("SQL create table requires a named dialect renderer")
	}
	defined := make(map[string]struct{}, len(b.columns))
	parts := make([]string, 0, len(b.columns)+len(b.constraints))
	for _, column := range b.columns {
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
