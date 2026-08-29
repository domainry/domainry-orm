package dialect

import (
	"fmt"
	"strings"
)

type Name string

const (
	SQLite   Name = "sqlite"
	MySQL    Name = "mysql"
	Postgres Name = "postgres"
)

// Dialect contains only portable SQL rendering behavior. Opening drivers,
// connection policy, capability probes, and migrations are separate concerns.
type Dialect struct {
	name  Name
	quote string
}

func New(name Name) (Dialect, error) {
	switch name {
	case SQLite:
		return Dialect{name: SQLite, quote: `"`}, nil
	case MySQL:
		return Dialect{name: MySQL, quote: "`"}, nil
	case Postgres:
		return Dialect{name: Postgres, quote: `"`}, nil
	default:
		return Dialect{}, fmt.Errorf("unsupported SQL dialect %q", name)
	}
}

func Parse(name string) (Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "sqlite", "sqlite3":
		return New(SQLite)
	case "mysql":
		return New(MySQL)
	case "postgres", "postgresql", "pgx":
		return New(Postgres)
	default:
		return Dialect{}, fmt.Errorf("unsupported SQL dialect %q", name)
	}
}

func (d Dialect) Name() Name { return d.name }

func (d Dialect) Identifier(value string) string {
	return QuoteIdentifier(value, d.quote)
}

func QuoteIdentifier(value, quote string) string {
	if !ValidIdentifier(value) {
		panic(fmt.Sprintf("invalid SQL identifier %q", value))
	}
	if quote != `"` && quote != "`" {
		panic(fmt.Sprintf("unsupported SQL identifier quote %q", quote))
	}
	return quote + value + quote
}

func QuestionPlaceholder(_ int) string { return "?" }

func (d Dialect) Table(schema, table string) string {
	if strings.TrimSpace(schema) == "" || d.name != Postgres {
		return d.Identifier(table)
	}
	return d.Identifier(schema) + "." + d.Identifier(table)
}

func (d Dialect) Placeholder(position int) string {
	if position <= 0 {
		panic(fmt.Sprintf("invalid SQL placeholder position %d", position))
	}
	if d.name == Postgres {
		return fmt.Sprintf("$%d", position)
	}
	return QuestionPlaceholder(position)
}

func (d Dialect) Placeholders(count int) []string {
	if count < 0 {
		panic(fmt.Sprintf("invalid SQL placeholder count %d", count))
	}
	values := make([]string, count)
	for index := range values {
		values[index] = d.Placeholder(index + 1)
	}
	return values
}

func (d Dialect) Columns(columns ...string) string {
	quoted := make([]string, len(columns))
	for index, column := range columns {
		quoted[index] = d.Identifier(column)
	}
	return strings.Join(quoted, ", ")
}

func (d Dialect) Insert(schema, table string, columns ...string) string {
	return "INSERT INTO " + d.Table(schema, table) + " (" + d.Columns(columns...) + ") VALUES (" + strings.Join(d.Placeholders(len(columns)), ", ") + ")"
}

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
