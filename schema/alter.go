package schema

import (
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

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
