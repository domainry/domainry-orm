package builder

import (
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

// -----------------------------------------------------------------------------
// IN / NOT IN against a subquery
// -----------------------------------------------------------------------------

type subquerySetPredicate struct {
	expression Expression
	subquery   *SelectBuilder
	not        bool
}

func InSubquery(column string, subquery *SelectBuilder) Predicate {
	return subquerySetPredicate{expression: Column(column), subquery: subquery}
}
func NotInSubquery(column string, subquery *SelectBuilder) Predicate {
	return subquerySetPredicate{expression: Column(column), subquery: subquery, not: true}
}
func InSubqueryExpression(expression Expression, subquery *SelectBuilder) Predicate {
	return subquerySetPredicate{expression: expression, subquery: subquery}
}
func NotInSubqueryExpression(expression Expression, subquery *SelectBuilder) Predicate {
	return subquerySetPredicate{expression: expression, subquery: subquery, not: true}
}

func (p subquerySetPredicate) renderPredicate(context *renderContext) (string, error) {
	if p.expression == nil || p.subquery == nil {
		return "", fmt.Errorf("SQL subquery set predicate requires expression and subquery")
	}
	left, err := p.expression.renderExpression(context)
	if err != nil {
		return "", err
	}
	inner, err := p.subquery.render(context, false)
	if err != nil {
		return "", err
	}
	operator := " IN "
	if p.not {
		operator = " NOT IN "
	}
	return left + operator + "(" + inner + ")", nil
}

// -----------------------------------------------------------------------------
// Comparison against a scalar subquery
// -----------------------------------------------------------------------------

type subqueryComparisonPredicate struct {
	left     Expression
	operator string
	subquery *SelectBuilder
}

func EqualSubquery(left Expression, subquery *SelectBuilder) Predicate {
	return subqueryComparisonPredicate{left: left, operator: "=", subquery: subquery}
}
func NotEqualSubquery(left Expression, subquery *SelectBuilder) Predicate {
	return subqueryComparisonPredicate{left: left, operator: "<>", subquery: subquery}
}
func LessThanSubquery(left Expression, subquery *SelectBuilder) Predicate {
	return subqueryComparisonPredicate{left: left, operator: "<", subquery: subquery}
}
func LessThanOrEqualSubquery(left Expression, subquery *SelectBuilder) Predicate {
	return subqueryComparisonPredicate{left: left, operator: "<=", subquery: subquery}
}
func GreaterThanSubquery(left Expression, subquery *SelectBuilder) Predicate {
	return subqueryComparisonPredicate{left: left, operator: ">", subquery: subquery}
}
func GreaterThanOrEqualSubquery(left Expression, subquery *SelectBuilder) Predicate {
	return subqueryComparisonPredicate{left: left, operator: ">=", subquery: subquery}
}

func (p subqueryComparisonPredicate) renderPredicate(context *renderContext) (string, error) {
	if p.left == nil || p.subquery == nil {
		return "", fmt.Errorf("SQL subquery comparison requires expression and subquery")
	}
	left, err := p.left.renderExpression(context)
	if err != nil {
		return "", err
	}
	inner, err := p.subquery.render(context, false)
	if err != nil {
		return "", err
	}
	return left + " " + p.operator + " (" + inner + ")", nil
}

// -----------------------------------------------------------------------------
// EXISTS / NOT EXISTS against a SelectBuilder subquery
// -----------------------------------------------------------------------------

type existsSubqueryPredicate struct {
	subquery *SelectBuilder
	not      bool
}

func ExistsSubquery(subquery *SelectBuilder) Predicate {
	return existsSubqueryPredicate{subquery: subquery}
}
func NotExistsSubquery(subquery *SelectBuilder) Predicate {
	return existsSubqueryPredicate{subquery: subquery, not: true}
}

func (p existsSubqueryPredicate) renderPredicate(context *renderContext) (string, error) {
	if p.subquery == nil {
		return "", fmt.Errorf("SQL exists predicate requires a subquery")
	}
	inner, err := p.subquery.render(context, false)
	if err != nil {
		return "", err
	}
	keyword := "EXISTS"
	if p.not {
		keyword = "NOT EXISTS"
	}
	return keyword + " (" + inner + ")", nil
}

// -----------------------------------------------------------------------------
// Case-insensitive LIKE (dialect-aware)
// -----------------------------------------------------------------------------

type iLikePredicate struct {
	expression Expression
	column     string
	pattern    string
	not        bool
}

func ILike(column, pattern string) Predicate {
	return iLikePredicate{column: column, pattern: pattern}
}
func NotILike(column, pattern string) Predicate {
	return iLikePredicate{column: column, pattern: pattern, not: true}
}
func ILikeExpression(expression Expression, pattern string) Predicate {
	return iLikePredicate{expression: expression, pattern: pattern}
}
func NotILikeExpression(expression Expression, pattern string) Predicate {
	return iLikePredicate{expression: expression, pattern: pattern, not: true}
}

func (p iLikePredicate) renderPredicate(context *renderContext) (string, error) {
	var operand string
	if p.expression != nil {
		rendered, err := p.expression.renderExpression(context)
		if err != nil {
			return "", err
		}
		operand = rendered
	} else {
		if strings.TrimSpace(p.column) == "" {
			return "", fmt.Errorf("SQL ilike predicate requires a column or expression")
		}
		operand = context.renderer.Identifier(p.column)
	}

	// PostgreSQL has native ILIKE. MySQL/SQLite emulate case-insensitivity by
	// lowering both sides.
	if name, _ := context.dialect(); name == dialect.Postgres {
		operator := " ILIKE "
		if p.not {
			operator = " NOT ILIKE "
		}
		return operand + operator + context.argument(p.pattern), nil
	}
	operator := " LIKE "
	if p.not {
		operator = " NOT LIKE "
	}
	return "LOWER(" + operand + ")" + operator + "LOWER(" + context.argument(p.pattern) + ")", nil
}
