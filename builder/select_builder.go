package builder

import (
	"fmt"
	"strings"
)

type SelectBuilder struct {
	renderer  Renderer
	table     string
	columns   []string
	predicate Predicate
	orders    []Order
	limit     int
	offset    int
	distinct  bool
}

func NewSelectBuilder(renderer Renderer, table string) *SelectBuilder {
	return &SelectBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}

func (b *SelectBuilder) Columns(columns ...string) *SelectBuilder {
	b.columns = append([]string(nil), columns...)
	return b
}
func (b *SelectBuilder) Distinct() *SelectBuilder                 { b.distinct = true; return b }
func (b *SelectBuilder) Where(predicate Predicate) *SelectBuilder { b.predicate = predicate; return b }
func (b *SelectBuilder) OrderBy(orders ...Order) *SelectBuilder {
	b.orders = append([]Order(nil), orders...)
	return b
}
func (b *SelectBuilder) Limit(limit int) *SelectBuilder   { b.limit = limit; return b }
func (b *SelectBuilder) Offset(offset int) *SelectBuilder { b.offset = offset; return b }

func (b *SelectBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" || len(b.columns) == 0 {
		return "", nil, fmt.Errorf("SQL select requires renderer, table, and columns")
	}
	context := &renderContext{renderer: b.renderer}
	columns := make([]string, len(b.columns))
	for index, column := range b.columns {
		columns[index] = b.renderer.Identifier(column)
	}
	keyword := "SELECT "
	if b.distinct {
		keyword += "DISTINCT "
	}
	statement := keyword + strings.Join(columns, ", ") + " FROM " + b.renderer.Table(b.table)
	if b.predicate != nil {
		where, err := b.predicate.renderPredicate(context)
		if err != nil {
			return "", nil, err
		}
		statement += " WHERE " + where
	}
	if len(b.orders) > 0 {
		orders := make([]string, len(b.orders))
		for index, order := range b.orders {
			rendered, err := order.render(b.renderer)
			if err != nil {
				return "", nil, err
			}
			orders[index] = rendered
		}
		statement += " ORDER BY " + strings.Join(orders, ", ")
	}
	if b.limit < 0 || b.offset < 0 {
		return "", nil, fmt.Errorf("SQL select limit and offset cannot be negative")
	}
	if b.limit > 0 {
		statement += " LIMIT " + context.argument(b.limit)
	}
	if b.offset > 0 {
		statement += " OFFSET " + context.argument(b.offset)
	}
	return statement, append([]any(nil), context.args...), nil
}
