package schema

import (
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

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
