package dialect

import (
	"fmt"
	"strings"
)

func (d Dialect) WithSchema(schema string) Renderer {
	return Renderer{dialect: d, schema: strings.TrimSpace(schema)}
}

// WithNamespace binds a schema and table prefix for stores that own their
// physical table namespace. Unlike WithSchema, an explicit schema is honored
// by every supported database because MySQL databases and SQLite attached
// databases also use schema-qualified table names.
func (d Dialect) WithNamespace(schema, tablePrefix string) (Renderer, error) {
	schema, tablePrefix = strings.TrimSpace(schema), strings.TrimSpace(tablePrefix)
	if schema != "" && !ValidIdentifier(schema) {
		return Renderer{}, fmt.Errorf("invalid SQL schema %q", schema)
	}
	if tablePrefix != "" && !ValidIdentifier(tablePrefix) {
		return Renderer{}, fmt.Errorf("invalid SQL table prefix %q", tablePrefix)
	}
	return Renderer{dialect: d, schema: schema, tablePrefix: tablePrefix, qualifySchema: true}, nil
}

func ParseRenderer(name, schema, tablePrefix string) (Renderer, error) {
	d, err := Parse(name)
	if err != nil {
		return Renderer{}, err
	}
	return d.WithNamespace(schema, tablePrefix)
}

// Renderer binds an optional schema to a dialect. Its method set is suitable
// for injection into modules that borrow a host-owned database.
type Renderer struct {
	dialect       Dialect
	schema        string
	tablePrefix   string
	qualifySchema bool
}

func (r Renderer) Name() Name                     { return r.dialect.Name() }
func (r Renderer) Identifier(value string) string { return r.dialect.Identifier(value) }
func (r Renderer) Table(value string) string {
	table := r.dialect.Identifier(r.tablePrefix + value)
	if r.schema != "" && (r.qualifySchema || r.dialect.Name() == Postgres) {
		return r.dialect.Identifier(r.schema) + "." + table
	}
	return table
}
func (r Renderer) Placeholder(position int) string { return r.dialect.Placeholder(position) }
func (r Renderer) Columns(values ...string) string { return r.dialect.Columns(values...) }
func (r Renderer) Insert(table string, columns []string) string {
	return "INSERT INTO " + r.Table(table) + " (" + r.Columns(columns...) + ") VALUES (" + strings.Join(r.dialect.Placeholders(len(columns)), ", ") + ")"
}

func ValidIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if index == 0 {
			if isASCIIAlpha(character) || character == '_' {
				continue
			}
			return false
		}
		if isASCIIAlpha(character) || character == '_' || character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

func isASCIIAlpha(character rune) bool {
	return character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}
