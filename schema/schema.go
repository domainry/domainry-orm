package schema

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

func Text() ColumnType      { return portableColumnType{name: "TEXT"} }
func LongText() ColumnType  { return portableColumnType{name: "LONGTEXT"} }
func BigInt() ColumnType    { return portableColumnType{name: "BIGINT"} }
func Integer() ColumnType   { return portableColumnType{name: "INTEGER"} }
func SmallInt() ColumnType  { return portableColumnType{name: "SMALLINT"} }
func Boolean() ColumnType   { return portableColumnType{name: "BOOLEAN"} }
func Real() ColumnType      { return portableColumnType{name: "REAL"} }
func Double() ColumnType    { return portableColumnType{name: "DOUBLE"} }
func Date() ColumnType      { return portableColumnType{name: "DATE"} }
func Time() ColumnType      { return portableColumnType{name: "TIME"} }
func Timestamp() ColumnType { return portableColumnType{name: "TIMESTAMP"} }
func JSON() ColumnType      { return portableColumnType{name: "JSON"} }
func UUID() ColumnType      { return portableColumnType{name: "UUID"} }
func Binary() ColumnType    { return portableColumnType{name: "BINARY"} }

func Decimal(precision, scale int) ColumnType {
	return portableColumnType{name: "DECIMAL", precision: precision, scale: scale}
}

func Varchar(length int) ColumnType {
	return portableColumnType{name: "VARCHAR", length: length}
}

// TextKey is an index-safe text identity. MySQL requires a bounded
// VARCHAR; PostgreSQL and SQLite use TEXT without leaking that distinction to
// schema owners.
func TextKey(maxLength int) ColumnType {
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
	case "LONGTEXT":
		if name == dialect.MySQL {
			return "LONGTEXT", nil
		}
		return "TEXT", nil
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

type ColumnDefinition struct {
	name              string
	typeOf            ColumnType
	notNull           bool
	defaultKeyword    string
	defaultLiteral    any
	hasDefaultLiteral bool
	characterSet      string
	collation         string
}

func Column(name string, typeOf ColumnType) ColumnDefinition {
	return ColumnDefinition{name: strings.TrimSpace(name), typeOf: typeOf}
}

func (c ColumnDefinition) NotNull() ColumnDefinition {
	c.notNull = true
	return c
}

// Default sets a portable, trusted SQL default. The accepted values are kept
// deliberately closed so schema construction cannot become a raw-SQL escape.
func (c ColumnDefinition) Default(value string) ColumnDefinition {
	c.defaultKeyword = strings.TrimSpace(value)
	c.defaultLiteral, c.hasDefaultLiteral = nil, false
	return c
}

// DefaultValue renders a typed DDL literal. Values are never interpreted as
// SQL expressions, so schema owners do not need a raw-SQL escape hatch for
// string, boolean, or numeric defaults.
func (c ColumnDefinition) DefaultValue(value any) ColumnDefinition {
	c.defaultKeyword = ""
	c.defaultLiteral, c.hasDefaultLiteral = value, true
	return c
}

// CharacterSet and Collation are identifier-only MySQL column attributes.
// Other dialects reject them instead of silently changing physical semantics.
func (c ColumnDefinition) CharacterSet(value string) ColumnDefinition {
	c.characterSet = strings.TrimSpace(value)
	return c
}

func (c ColumnDefinition) Collation(value string) ColumnDefinition {
	c.collation = strings.TrimSpace(value)
	return c
}

type tableConstraint struct {
	kind    string
	columns []string
}

type TableBuilder struct {
	renderer        Renderer
	table           string
	ifNotExists     bool
	columns         []ColumnDefinition
	requiredColumns []ColumnDefinition
	constraints     []tableConstraint
}

func NewTable(renderer Renderer, table string) *TableBuilder {
	return &TableBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}

// NewTableWithRequiredColumns creates a table declaration whose convention-
// owned columns cannot be omitted accidentally. Convention packages such as
// recordschema use this without moving their policy into the generic schema
// package.
func NewTableWithRequiredColumns(renderer Renderer, table string, columns ...ColumnDefinition) *TableBuilder {
	return &TableBuilder{
		renderer:        renderer,
		table:           strings.TrimSpace(table),
		requiredColumns: append([]ColumnDefinition(nil), columns...),
	}
}

func (b *TableBuilder) IfNotExists() *TableBuilder {
	b.ifNotExists = true
	return b
}

// WithoutRequiredColumns removes convention-owned columns. It exists for
// compatibility facades; new infrastructure declarations should start with
// NewTable instead of opting out of a convention after construction.
func (b *TableBuilder) WithoutRequiredColumns() *TableBuilder {
	b.requiredColumns = nil
	return b
}

// WithoutSystemColumns is retained for the legacy builder facade. New code
// should construct infrastructure tables with schema.NewTable directly.
// Deprecated: use NewTable.
func (b *TableBuilder) WithoutSystemColumns() *TableBuilder {
	return b.WithoutRequiredColumns()
}

func (b *TableBuilder) Columns(columns ...ColumnDefinition) *TableBuilder {
	b.columns = append([]ColumnDefinition(nil), columns...)
	return b
}

func (b *TableBuilder) PrimaryKey(columns ...string) *TableBuilder {
	b.constraints = append(b.constraints, tableConstraint{kind: "PRIMARY KEY", columns: append([]string(nil), columns...)})
	return b
}

func (b *TableBuilder) Unique(columns ...string) *TableBuilder {
	b.constraints = append(b.constraints, tableConstraint{kind: "UNIQUE", columns: append([]string(nil), columns...)})
	return b
}

func (b *TableBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" {
		return "", nil, fmt.Errorf("SQL create table requires renderer and table")
	}
	name, ok := b.renderer.(namedRenderer)
	if !ok {
		return "", nil, fmt.Errorf("SQL create table requires a named dialect renderer")
	}
	columns, err := mergeRequiredColumns(b.columns, b.requiredColumns)
	if err != nil {
		return "", nil, err
	}
	if len(columns) == 0 {
		return "", nil, fmt.Errorf("SQL table requires columns")
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
		if column.characterSet != "" || column.collation != "" {
			if name.Name() != dialect.MySQL {
				return "", nil, fmt.Errorf("SQL table column %q character set and collation require MySQL", column.name)
			}
			if column.characterSet != "" {
				if !dialect.ValidIdentifier(column.characterSet) {
					return "", nil, fmt.Errorf("SQL table column %q has invalid character set", column.name)
				}
				definition += " CHARACTER SET " + column.characterSet
			}
			if column.collation != "" {
				if !dialect.ValidIdentifier(column.collation) {
					return "", nil, fmt.Errorf("SQL table column %q has invalid collation", column.name)
				}
				definition += " COLLATE " + column.collation
			}
		}
		if column.notNull {
			definition += " NOT NULL"
		}
		if column.defaultKeyword != "" {
			value, err := renderColumnDefault(name.Name(), column.defaultKeyword)
			if err != nil {
				return "", nil, fmt.Errorf("SQL table column %q: %w", column.name, err)
			}
			definition += " DEFAULT " + value
		} else if column.hasDefaultLiteral {
			value, err := renderColumnDefaultLiteral(name.Name(), column.defaultLiteral)
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

type PhysicalTable struct {
	Name    string
	Columns []PhysicalColumn
}

type PhysicalColumn struct {
	Name       string
	Type       string
	Nullable   bool
	PrimaryKey bool
}

// PhysicalTable projects the exact physical table shape from the same
// portable declaration used by Build. Embedded modules use it to prove and
// adopt source-owned tables that predate module extraction without duplicating
// dialect-specific column-type knowledge.
func (b *TableBuilder) PhysicalTable() (PhysicalTable, error) {
	if b == nil || b.renderer == nil || b.table == "" {
		return PhysicalTable{}, fmt.Errorf("SQL physical table requires renderer and table")
	}
	named, ok := b.renderer.(namedRenderer)
	if !ok {
		return PhysicalTable{}, fmt.Errorf("SQL physical table requires a named dialect renderer")
	}
	columns, err := mergeRequiredColumns(b.columns, b.requiredColumns)
	if err != nil {
		return PhysicalTable{}, err
	}
	primary := map[string]struct{}{}
	for _, constraint := range b.constraints {
		if constraint.kind == "PRIMARY KEY" {
			for _, column := range constraint.columns {
				primary[strings.ToLower(strings.TrimSpace(column))] = struct{}{}
			}
		}
	}
	result := PhysicalTable{Name: b.table, Columns: make([]PhysicalColumn, len(columns))}
	for index, column := range columns {
		if column.name == "" || column.typeOf == nil {
			return PhysicalTable{}, fmt.Errorf("SQL physical table column requires name and type")
		}
		physical, err := column.typeOf.renderColumnType(named.Name())
		if err != nil {
			return PhysicalTable{}, err
		}
		_, isPrimary := primary[strings.ToLower(column.name)]
		result.Columns[index] = PhysicalColumn{Name: column.name, Type: physical, Nullable: !column.notNull, PrimaryKey: isPrimary}
	}
	return result, nil
}

func mergeRequiredColumns(columns, requiredColumns []ColumnDefinition) ([]ColumnDefinition, error) {
	result := append([]ColumnDefinition(nil), columns...)
	existing := make(map[string]ColumnDefinition, len(columns))
	for _, column := range columns {
		key := strings.ToLower(column.name)
		if _, found := existing[key]; found {
			return nil, fmt.Errorf("SQL table column %q is duplicated", column.name)
		}
		existing[key] = column
	}
	for _, required := range requiredColumns {
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

func renderColumnDefaultLiteral(name dialect.Name, value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return "'" + strings.ReplaceAll(typed, "'", "''") + "'", nil
	case bool:
		if name == dialect.SQLite {
			if typed {
				return "1", nil
			}
			return "0", nil
		}
		if typed {
			return "TRUE", nil
		}
		return "FALSE", nil
	case int:
		return strconv.Itoa(typed), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	default:
		return "", fmt.Errorf("unsupported SQL column default literal %T", value)
	}
}

type AddColumnBuilder struct {
	renderer Renderer
	table    string
	column   ColumnDefinition
}

func NewAddColumn(renderer Renderer, table string, column ColumnDefinition) *AddColumnBuilder {
	return &AddColumnBuilder{renderer: renderer, table: strings.TrimSpace(table), column: column}
}

func (b *AddColumnBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" || b.column.name == "" || b.column.typeOf == nil {
		return "", nil, fmt.Errorf("SQL add column requires renderer, table, and column")
	}
	name, ok := b.renderer.(namedRenderer)
	if !ok {
		return "", nil, fmt.Errorf("SQL add column requires a named dialect renderer")
	}
	columnType, err := b.column.typeOf.renderColumnType(name.Name())
	if err != nil {
		return "", nil, err
	}
	definition := b.renderer.Identifier(b.column.name) + " " + columnType
	if b.column.characterSet != "" || b.column.collation != "" {
		if name.Name() != dialect.MySQL {
			return "", nil, fmt.Errorf("SQL table column %q character set and collation require MySQL", b.column.name)
		}
		if b.column.characterSet != "" {
			if !dialect.ValidIdentifier(b.column.characterSet) {
				return "", nil, fmt.Errorf("SQL table column %q has invalid character set", b.column.name)
			}
			definition += " CHARACTER SET " + b.column.characterSet
		}
		if b.column.collation != "" {
			if !dialect.ValidIdentifier(b.column.collation) {
				return "", nil, fmt.Errorf("SQL table column %q has invalid collation", b.column.name)
			}
			definition += " COLLATE " + b.column.collation
		}
	}
	if b.column.notNull {
		definition += " NOT NULL"
	}
	if b.column.defaultKeyword != "" {
		value, err := renderColumnDefault(name.Name(), b.column.defaultKeyword)
		if err != nil {
			return "", nil, fmt.Errorf("SQL table column %q: %w", b.column.name, err)
		}
		definition += " DEFAULT " + value
	} else if b.column.hasDefaultLiteral {
		value, err := renderColumnDefaultLiteral(name.Name(), b.column.defaultLiteral)
		if err != nil {
			return "", nil, fmt.Errorf("SQL table column %q: %w", b.column.name, err)
		}
		definition += " DEFAULT " + value
	}
	return "ALTER TABLE " + b.renderer.Table(b.table) + " ADD COLUMN " + definition, []any{}, nil
}

type RenameColumnBuilder struct {
	renderer Renderer
	table    string
	from     string
	to       string
}

type DropColumnBuilder struct {
	renderer Renderer
	table    string
	column   string
}

func NewDropColumn(renderer Renderer, table, column string) *DropColumnBuilder {
	return &DropColumnBuilder{renderer: renderer, table: strings.TrimSpace(table), column: strings.TrimSpace(column)}
}

func (b *DropColumnBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" || b.column == "" {
		return "", nil, fmt.Errorf("SQL drop column requires renderer, table, and column")
	}
	return "ALTER TABLE " + b.renderer.Table(b.table) + " DROP COLUMN " + b.renderer.Identifier(b.column), []any{}, nil
}

func NewRenameColumn(renderer Renderer, table, from, to string) *RenameColumnBuilder {
	return &RenameColumnBuilder{
		renderer: renderer,
		table:    strings.TrimSpace(table),
		from:     strings.TrimSpace(from),
		to:       strings.TrimSpace(to),
	}
}

func (b *RenameColumnBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" || b.from == "" || b.to == "" {
		return "", nil, fmt.Errorf("SQL rename column requires renderer, table, source, and target")
	}
	if strings.EqualFold(b.from, b.to) {
		return "", nil, fmt.Errorf("SQL rename column source and target must differ")
	}
	return "ALTER TABLE " + b.renderer.Table(b.table) + " RENAME COLUMN " + b.renderer.Identifier(b.from) + " TO " + b.renderer.Identifier(b.to), []any{}, nil
}

type IndexBuilder struct {
	renderer    Renderer
	index       string
	table       string
	columns     []string
	unique      bool
	ifNotExists bool
}

func NewIndex(renderer Renderer, index, table string) *IndexBuilder {
	return &IndexBuilder{renderer: renderer, index: strings.TrimSpace(index), table: strings.TrimSpace(table)}
}

func (b *IndexBuilder) Columns(columns ...string) *IndexBuilder {
	b.columns = append([]string(nil), columns...)
	return b
}

func (b *IndexBuilder) Unique() *IndexBuilder {
	b.unique = true
	return b
}

func (b *IndexBuilder) IfNotExists() *IndexBuilder {
	b.ifNotExists = true
	return b
}

func (b *IndexBuilder) Build() (string, []any, error) {
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
