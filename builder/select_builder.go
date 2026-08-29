package builder

import (
	"fmt"
	"strings"
)

type SelectBuilder struct {
	renderer    Renderer
	table       string
	columns     []string
	projections []Projection
	alias       string
	joins       []Join
	predicate   Predicate
	groups      []Expression
	having      Predicate
	orders      []Order
	limit       int
	offset      int
	distinct    bool
}

func NewSelectBuilder(renderer Renderer, table string) *SelectBuilder {
	return &SelectBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}

func (b *SelectBuilder) Columns(columns ...string) *SelectBuilder {
	b.columns = append([]string(nil), columns...)
	b.projections = nil
	return b
}
func (b *SelectBuilder) Projections(projections ...Projection) *SelectBuilder {
	b.projections = append([]Projection(nil), projections...)
	b.columns = nil
	return b
}
func (b *SelectBuilder) Alias(alias string) *SelectBuilder {
	b.alias = strings.TrimSpace(alias)
	return b
}
func (b *SelectBuilder) Join(joins ...Join) *SelectBuilder {
	b.joins = append([]Join(nil), joins...)
	return b
}
func (b *SelectBuilder) GroupBy(expressions ...Expression) *SelectBuilder {
	b.groups = append([]Expression(nil), expressions...)
	return b
}
func (b *SelectBuilder) Having(predicate Predicate) *SelectBuilder { b.having = predicate; return b }
func (b *SelectBuilder) Distinct() *SelectBuilder                  { b.distinct = true; return b }
func (b *SelectBuilder) Where(predicate Predicate) *SelectBuilder  { b.predicate = predicate; return b }
func (b *SelectBuilder) OrderBy(orders ...Order) *SelectBuilder {
	b.orders = append([]Order(nil), orders...)
	return b
}
func (b *SelectBuilder) Limit(limit int) *SelectBuilder   { b.limit = limit; return b }
func (b *SelectBuilder) Offset(offset int) *SelectBuilder { b.offset = offset; return b }

func (b *SelectBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" || len(b.columns) == 0 && len(b.projections) == 0 {
		return "", nil, fmt.Errorf("SQL select requires renderer, table, and columns")
	}
	context := &renderContext{renderer: b.renderer}
	columns := make([]string, 0, max(len(b.columns), len(b.projections)))
	for _, column := range b.columns {
		columns = append(columns, b.renderer.Identifier(column))
	}
	for _, projection := range b.projections {
		rendered, err := projection.render(context)
		if err != nil {
			return "", nil, err
		}
		columns = append(columns, rendered)
	}
	keyword := "SELECT "
	if b.distinct {
		keyword += "DISTINCT "
	}
	statement := keyword + strings.Join(columns, ", ") + " FROM " + b.renderer.Table(b.table)
	if b.alias != "" {
		statement += " AS " + b.renderer.Identifier(b.alias)
	}
	for _, join := range b.joins {
		rendered, err := join.render(context)
		if err != nil {
			return "", nil, err
		}
		statement += " " + rendered
	}
	if b.predicate != nil {
		where, err := b.predicate.renderPredicate(context)
		if err != nil {
			return "", nil, err
		}
		statement += " WHERE " + where
	}
	if len(b.groups) > 0 {
		groups := make([]string, len(b.groups))
		for index, group := range b.groups {
			if group == nil {
				return "", nil, fmt.Errorf("SQL group expression is required")
			}
			rendered, err := group.renderExpression(context)
			if err != nil {
				return "", nil, err
			}
			groups[index] = rendered
		}
		statement += " GROUP BY " + strings.Join(groups, ", ")
	}
	if b.having != nil {
		having, err := b.having.renderPredicate(context)
		if err != nil {
			return "", nil, err
		}
		statement += " HAVING " + having
	}
	if len(b.orders) > 0 {
		orders := make([]string, len(b.orders))
		for index, order := range b.orders {
			rendered, err := order.render(context)
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
