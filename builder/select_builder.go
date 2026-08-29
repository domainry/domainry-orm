package builder

import (
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

type combination struct {
	operator string // UNION, UNION ALL, INTERSECT, EXCEPT, ...
	builder  *SelectBuilder
}

type SelectBuilder struct {
	renderer      Renderer
	table         string
	fromSubquery  *SelectBuilder
	fromAlias     string
	fromLateral   bool
	fromCTE       bool
	ctes          []cte
	columns       []string
	projections   []Projection
	distinctOn    []Expression
	alias         string
	joins         []Join
	predicate     Predicate
	groups        []Expression
	groupModifier string // ROLLUP, CUBE, GROUPING SETS
	having        Predicate
	orders        []Order
	limit         int
	offset        int
	distinct      bool
	locking       string
	combinations  []combination
	required      []Predicate
	workspaceMode bool
	buildError    error
}

type cte struct {
	name      string
	columns   []string
	recursive bool
	builder   *SelectBuilder
}

func NewSelectBuilder(renderer Renderer, table string) *SelectBuilder {
	return &SelectBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}

// NewSelectFromSubquery builds a SELECT whose FROM clause is a derived table
// (subquery) with a mandatory alias.
func NewSelectFromSubquery(renderer Renderer, subquery *SelectBuilder, alias string) *SelectBuilder {
	return &SelectBuilder{renderer: renderer, fromSubquery: subquery, fromAlias: strings.TrimSpace(alias)}
}

// NewSelectFromCTE selects from a common-table-expression name. Unlike a
// physical table, a CTE is rendered as an identifier and never receives the
// renderer's schema or table prefix.
func NewSelectFromCTE(renderer Renderer, name, alias string) *SelectBuilder {
	return &SelectBuilder{renderer: renderer, table: strings.TrimSpace(name), alias: strings.TrimSpace(alias), fromCTE: true}
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
	b.groupModifier = ""
	return b
}

// GroupByRollup, GroupByCube and GroupByGroupingSets wrap the grouping
// expressions with the corresponding SQL grouping modifier.
func (b *SelectBuilder) GroupByRollup(expressions ...Expression) *SelectBuilder {
	b.groups = append([]Expression(nil), expressions...)
	b.groupModifier = "ROLLUP"
	return b
}
func (b *SelectBuilder) GroupByCube(expressions ...Expression) *SelectBuilder {
	b.groups = append([]Expression(nil), expressions...)
	b.groupModifier = "CUBE"
	return b
}
func (b *SelectBuilder) GroupByGroupingSets(expressions ...Expression) *SelectBuilder {
	b.groups = append([]Expression(nil), expressions...)
	b.groupModifier = "GROUPING SETS"
	return b
}
func (b *SelectBuilder) Having(predicate Predicate) *SelectBuilder { b.having = predicate; return b }
func (b *SelectBuilder) Distinct() *SelectBuilder                  { b.distinct = true; return b }

// DistinctOn emits PostgreSQL's DISTINCT ON (expressions...).
func (b *SelectBuilder) DistinctOn(expressions ...Expression) *SelectBuilder {
	b.distinctOn = append([]Expression(nil), expressions...)
	b.distinct = false
	return b
}
func (b *SelectBuilder) Where(predicate Predicate) *SelectBuilder { b.predicate = predicate; return b }
func (b *SelectBuilder) OrderBy(orders ...Order) *SelectBuilder {
	b.orders = append([]Order(nil), orders...)
	return b
}
func (b *SelectBuilder) Limit(limit int) *SelectBuilder { b.limit = limit; return b }
func (b *SelectBuilder) Offset(offset int) *SelectBuilder {
	b.offset = offset
	if b.workspaceMode && offset > 0 {
		b.buildError = fmt.Errorf("SQL workspace pagination requires an ID cursor; OFFSET is not allowed")
	}
	return b
}

// AfterID applies stable ascending keyset pagination. Empty cursors fail the
// build so callers cannot silently fall back to an unbounded/deep OFFSET scan.
func (b *SelectBuilder) AfterID(id string) *SelectBuilder {
	id = strings.TrimSpace(id)
	if id == "" {
		b.buildError = fmt.Errorf("SQL keyset pagination requires an ID cursor")
		return b
	}
	b.required = append(b.required, GreaterThan("id", id))
	b.OrderBy(Ascending("id"))
	return b
}

// Lateral marks a subquery FROM source as LATERAL.
func (b *SelectBuilder) Lateral() *SelectBuilder { b.fromLateral = true; return b }

// ForUpdate, ForNoKeyUpdate, ForShare and ForKeyShare attach row-level locking
// clauses. Modifiers such as "NOWAIT" or "SKIP LOCKED" may be supplied.
func (b *SelectBuilder) ForUpdate(modifiers ...string) *SelectBuilder {
	b.locking = lockClause("FOR UPDATE", modifiers)
	return b
}
func (b *SelectBuilder) ForNoKeyUpdate(modifiers ...string) *SelectBuilder {
	b.locking = lockClause("FOR NO KEY UPDATE", modifiers)
	return b
}
func (b *SelectBuilder) ForShare(modifiers ...string) *SelectBuilder {
	b.locking = lockClause("FOR SHARE", modifiers)
	return b
}
func (b *SelectBuilder) ForKeyShare(modifiers ...string) *SelectBuilder {
	b.locking = lockClause("FOR KEY SHARE", modifiers)
	return b
}

func lockClause(base string, modifiers []string) string {
	clause := base
	for _, modifier := range modifiers {
		if trimmed := strings.ToUpper(strings.TrimSpace(modifier)); trimmed != "" {
			if trimmed != "NOWAIT" && trimmed != "SKIP LOCKED" {
				return "INVALID"
			}
			clause += " " + trimmed
		}
	}
	return clause
}

// Union, UnionAll, Intersect, IntersectAll, Except and ExceptAll append a set
// operation combining this query with another SELECT.
func (b *SelectBuilder) Union(other *SelectBuilder) *SelectBuilder {
	return b.combine("UNION", other)
}
func (b *SelectBuilder) UnionAll(other *SelectBuilder) *SelectBuilder {
	return b.combine("UNION ALL", other)
}
func (b *SelectBuilder) Intersect(other *SelectBuilder) *SelectBuilder {
	return b.combine("INTERSECT", other)
}
func (b *SelectBuilder) IntersectAll(other *SelectBuilder) *SelectBuilder {
	return b.combine("INTERSECT ALL", other)
}
func (b *SelectBuilder) Except(other *SelectBuilder) *SelectBuilder {
	return b.combine("EXCEPT", other)
}
func (b *SelectBuilder) ExceptAll(other *SelectBuilder) *SelectBuilder {
	return b.combine("EXCEPT ALL", other)
}
func (b *SelectBuilder) combine(operator string, other *SelectBuilder) *SelectBuilder {
	b.combinations = append(b.combinations, combination{operator: operator, builder: other})
	return b
}

// With and WithRecursive prepend common table expressions to the query.
func (b *SelectBuilder) With(name string, query *SelectBuilder, columns ...string) *SelectBuilder {
	b.ctes = append(b.ctes, cte{name: strings.TrimSpace(name), columns: columns, builder: query})
	return b
}
func (b *SelectBuilder) WithRecursive(name string, query *SelectBuilder, columns ...string) *SelectBuilder {
	b.ctes = append(b.ctes, cte{name: strings.TrimSpace(name), columns: columns, recursive: true, builder: query})
	return b
}

func (b *SelectBuilder) Build() (string, []any, error) {
	return b.BuildWithOffset(0)
}

// BuildWithOffset renders a structured SELECT that will be embedded after
// offset arguments already owned by a larger prepared statement. This keeps
// PostgreSQL placeholder numbering correct without exposing raw SQL.
func (b *SelectBuilder) BuildWithOffset(offset int) (string, []any, error) {
	if b == nil || b.renderer == nil {
		return "", nil, fmt.Errorf("SQL select requires renderer")
	}
	if offset < 0 {
		return "", nil, fmt.Errorf("SQL select argument offset cannot be negative")
	}
	context := &renderContext{renderer: b.renderer, offset: offset}
	statement, err := b.render(context, true)
	if err != nil {
		return "", nil, err
	}
	return statement, append([]any(nil), context.args...), nil
}

// render emits the SELECT into the shared context. When topLevel is false the
// caller is embedding the query (subquery, CTE, set operation) and the leading
// WITH clause is suppressed.
func (b *SelectBuilder) render(context *renderContext, topLevel bool) (string, error) {
	if b.buildError != nil {
		return "", b.buildError
	}
	if b.table == "" && b.fromSubquery == nil {
		return "", fmt.Errorf("SQL select requires a table or subquery source")
	}
	if len(b.columns) == 0 && len(b.projections) == 0 {
		return "", fmt.Errorf("SQL select requires columns")
	}
	if b.limit < 0 || b.offset < 0 {
		return "", fmt.Errorf("SQL select limit and offset cannot be negative")
	}

	var statement string
	if topLevel && len(b.ctes) > 0 {
		rendered, err := b.renderCTEs(context)
		if err != nil {
			return "", err
		}
		statement += rendered
	}

	keyword := "SELECT "
	if len(b.distinctOn) > 0 {
		on, err := renderExpressionList(context, b.distinctOn)
		if err != nil {
			return "", err
		}
		keyword += "DISTINCT ON (" + on + ") "
	} else if b.distinct {
		keyword += "DISTINCT "
	}

	columns := make([]string, 0, max(len(b.columns), len(b.projections)))
	for _, column := range b.columns {
		columns = append(columns, b.renderer.Identifier(column))
	}
	for _, projection := range b.projections {
		rendered, err := projection.render(context)
		if err != nil {
			return "", err
		}
		columns = append(columns, rendered)
	}

	from, err := b.renderFrom(context)
	if err != nil {
		return "", err
	}
	statement += keyword + strings.Join(columns, ", ") + " FROM " + from

	for _, join := range b.joins {
		rendered, err := join.render(context)
		if err != nil {
			return "", err
		}
		statement += " " + rendered
	}
	predicate := b.predicate
	if len(b.required) > 0 {
		parts := append([]Predicate(nil), b.required...)
		if predicate != nil {
			parts = append(parts, predicate)
		}
		predicate = And(parts...)
	}
	if predicate != nil {
		where, err := predicate.renderPredicate(context)
		if err != nil {
			return "", err
		}
		statement += " WHERE " + where
	}
	if len(b.groups) > 0 {
		groups, err := renderExpressionList(context, b.groups)
		if err != nil {
			return "", err
		}
		if b.groupModifier != "" {
			statement += " GROUP BY " + b.groupModifier + " (" + groups + ")"
		} else {
			statement += " GROUP BY " + groups
		}
	}
	if b.having != nil {
		having, err := b.having.renderPredicate(context)
		if err != nil {
			return "", err
		}
		statement += " HAVING " + having
	}

	// Set operations bind before ORDER BY/LIMIT, which then apply to the whole
	// combined result.
	for _, combined := range b.combinations {
		if combined.builder == nil {
			return "", fmt.Errorf("SQL set operation requires a query")
		}
		rendered, err := combined.builder.render(context, false)
		if err != nil {
			return "", err
		}
		statement += " " + combined.operator + " " + rendered
	}

	if len(b.orders) > 0 {
		orders := make([]string, len(b.orders))
		for index, order := range b.orders {
			rendered, err := order.render(context)
			if err != nil {
				return "", err
			}
			orders[index] = rendered
		}
		statement += " ORDER BY " + strings.Join(orders, ", ")
	}
	if b.limit > 0 {
		statement += " LIMIT " + context.argument(b.limit)
	}
	if b.offset > 0 {
		statement += " OFFSET " + context.argument(b.offset)
	}
	if b.locking != "" {
		if b.locking == "INVALID" {
			return "", fmt.Errorf("SQL row lock modifier is invalid")
		}
		statement += " " + b.locking
	}
	return statement, nil
}

func (b *SelectBuilder) renderFrom(context *renderContext) (string, error) {
	if b.fromSubquery != nil {
		if b.fromAlias == "" {
			return "", fmt.Errorf("SQL select subquery source requires an alias")
		}
		inner, err := b.fromSubquery.render(context, false)
		if err != nil {
			return "", err
		}
		prefix := ""
		if b.fromLateral {
			prefix = "LATERAL "
		}
		return prefix + "(" + inner + ") AS " + b.renderer.Identifier(b.fromAlias), nil
	}
	from := b.renderer.Table(b.table)
	if b.fromCTE {
		from = b.renderer.Identifier(b.table)
	}
	if b.alias != "" {
		from += " AS " + b.renderer.Identifier(b.alias)
	}
	return from, nil
}

func (b *SelectBuilder) renderCTEs(context *renderContext) (string, error) {
	recursive := false
	parts := make([]string, len(b.ctes))
	for index, item := range b.ctes {
		if item.name == "" || item.builder == nil {
			return "", fmt.Errorf("SQL common table expression requires a name and query")
		}
		if item.recursive {
			recursive = true
		}
		inner, err := item.builder.render(context, false)
		if err != nil {
			return "", err
		}
		head := b.renderer.Identifier(item.name)
		if len(item.columns) > 0 {
			quoted := make([]string, len(item.columns))
			for i, column := range item.columns {
				quoted[i] = b.renderer.Identifier(column)
			}
			head += " (" + strings.Join(quoted, ", ") + ")"
		}
		parts[index] = head + " AS (" + inner + ")"
	}
	keyword := "WITH "
	if recursive {
		keyword = "WITH RECURSIVE "
	}
	return keyword + strings.Join(parts, ", ") + " ", nil
}

func renderExpressionList(context *renderContext, expressions []Expression) (string, error) {
	parts := make([]string, len(expressions))
	for index, expression := range expressions {
		if expression == nil {
			return "", fmt.Errorf("SQL expression is required")
		}
		rendered, err := expression.renderExpression(context)
		if err != nil {
			return "", err
		}
		parts[index] = rendered
	}
	return strings.Join(parts, ", "), nil
}

// ensure dialect import is retained for the Renderer contract.
var _ = dialect.Postgres
