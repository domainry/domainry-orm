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
