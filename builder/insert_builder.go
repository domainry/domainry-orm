package builder

import (
	"fmt"
	"strings"
)

type InsertBuilder struct {
	renderer Renderer
	table    string
	columns  []string
	rows     [][]any
}

func NewInsertBuilder(renderer Renderer, table string) *InsertBuilder {
	return &InsertBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}
func (b *InsertBuilder) Columns(columns ...string) *InsertBuilder {
	b.columns = append([]string(nil), columns...)
	return b
}
func (b *InsertBuilder) Values(values ...any) *InsertBuilder {
	b.rows = append(b.rows, append([]any(nil), values...))
	return b
}

func (b *InsertBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" || len(b.columns) == 0 || len(b.rows) == 0 {
		return "", nil, fmt.Errorf("SQL insert requires renderer, table, columns, and values")
	}
	context := &renderContext{renderer: b.renderer}
	columns := make([]string, len(b.columns))
	for index, column := range b.columns {
		columns[index] = b.renderer.Identifier(column)
	}
	rows := make([]string, len(b.rows))
	for rowIndex, row := range b.rows {
		if len(row) != len(b.columns) {
			return "", nil, fmt.Errorf("SQL insert row %d has %d values for %d columns", rowIndex, len(row), len(b.columns))
		}
		placeholders := make([]string, len(row))
		for index, value := range row {
			placeholders[index] = context.argument(value)
		}
		rows[rowIndex] = "(" + strings.Join(placeholders, ", ") + ")"
	}
	statement := "INSERT INTO " + b.renderer.Table(b.table) + " (" + strings.Join(columns, ", ") + ") VALUES " + strings.Join(rows, ", ")
	return statement, append([]any(nil), context.args...), nil
}
