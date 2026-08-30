package schema

import (
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

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
