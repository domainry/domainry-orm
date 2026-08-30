package schema

import "strings"

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
