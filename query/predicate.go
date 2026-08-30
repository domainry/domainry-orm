package query

import (
	"fmt"
	"strings"
)

type Predicate interface {
	renderPredicate(*renderContext) (string, error)
}

type comparisonPredicate struct {
	column   string
	operator string
	value    any
}

func Equal(column string, value any) Predicate {
	return comparisonPredicate{column: column, operator: "=", value: value}
}
func NotEqual(column string, value any) Predicate {
	return comparisonPredicate{column: column, operator: "<>", value: value}
}
func LessThan(column string, value any) Predicate {
	return comparisonPredicate{column: column, operator: "<", value: value}
}
func LessThanOrEqual(column string, value any) Predicate {
	return comparisonPredicate{column: column, operator: "<=", value: value}
}
func GreaterThan(column string, value any) Predicate {
	return comparisonPredicate{column: column, operator: ">", value: value}
}
func GreaterThanOrEqual(column string, value any) Predicate {
	return comparisonPredicate{column: column, operator: ">=", value: value}
}

func (p comparisonPredicate) renderPredicate(context *renderContext) (string, error) {
	if strings.TrimSpace(p.column) == "" {
		return "", fmt.Errorf("SQL comparison column is required")
	}
	return context.renderer.Identifier(p.column) + " " + p.operator + " " + context.argument(p.value), nil
}

type expressionComparisonPredicate struct {
	left     Expression
	operator string
	right    Expression
}

func EqualExpressions(left, right Expression) Predicate {
	return expressionComparisonPredicate{left: left, operator: "=", right: right}
}
func NotEqualExpressions(left, right Expression) Predicate {
	return expressionComparisonPredicate{left: left, operator: "<>", right: right}
}
func LessThanExpressions(left, right Expression) Predicate {
	return expressionComparisonPredicate{left: left, operator: "<", right: right}
}
func LessThanOrEqualExpressions(left, right Expression) Predicate {
	return expressionComparisonPredicate{left: left, operator: "<=", right: right}
}
func GreaterThanExpressions(left, right Expression) Predicate {
	return expressionComparisonPredicate{left: left, operator: ">", right: right}
}
func GreaterThanOrEqualExpressions(left, right Expression) Predicate {
	return expressionComparisonPredicate{left: left, operator: ">=", right: right}
}
func EqualValue(left Expression, value any) Predicate {
	return expressionComparisonPredicate{left: left, operator: "=", right: Value(value)}
}
func NotEqualValue(left Expression, value any) Predicate {
	return expressionComparisonPredicate{left: left, operator: "<>", right: Value(value)}
}
func LessThanValue(left Expression, value any) Predicate {
	return expressionComparisonPredicate{left: left, operator: "<", right: Value(value)}
}
func LessThanOrEqualValue(left Expression, value any) Predicate {
	return expressionComparisonPredicate{left: left, operator: "<=", right: Value(value)}
}
func GreaterThanExpression(left Expression, value any) Predicate {
	return expressionComparisonPredicate{left: left, operator: ">", right: Value(value)}
}
func GreaterThanOrEqualValue(left Expression, value any) Predicate {
	return expressionComparisonPredicate{left: left, operator: ">=", right: Value(value)}
}

func (p expressionComparisonPredicate) renderPredicate(context *renderContext) (string, error) {
	if p.left == nil || p.right == nil {
		return "", fmt.Errorf("SQL expression comparison operands are required")
	}
	left, err := p.left.renderExpression(context)
	if err != nil {
		return "", err
	}
	right, err := p.right.renderExpression(context)
	if err != nil {
		return "", err
	}
	return left + " " + p.operator + " " + right, nil
}

type betweenPredicate struct {
	column string
	lower  any
	upper  any
	not    bool
}

func Between(column string, lower, upper any) Predicate {
	return betweenPredicate{column: column, lower: lower, upper: upper}
}
func NotBetween(column string, lower, upper any) Predicate {
	return betweenPredicate{column: column, lower: lower, upper: upper, not: true}
}
func (p betweenPredicate) renderPredicate(context *renderContext) (string, error) {
	if strings.TrimSpace(p.column) == "" {
		return "", fmt.Errorf("SQL between predicate column is required")
	}
	operator := " BETWEEN "
	if p.not {
		operator = " NOT BETWEEN "
	}
	return context.renderer.Identifier(p.column) + operator + context.argument(p.lower) + " AND " + context.argument(p.upper), nil
}

type likePredicate struct {
	column  string
	pattern string
	not     bool
	escaped bool
}

func Like(column, pattern string) Predicate { return likePredicate{column: column, pattern: pattern} }
func NotLike(column, pattern string) Predicate {
	return likePredicate{column: column, pattern: pattern, not: true}
}
func LikeValue(expression Expression, pattern string) Predicate {
	return expressionLikePredicate{expression: expression, pattern: pattern}
}
func NotLikeValue(expression Expression, pattern string) Predicate {
	return expressionLikePredicate{expression: expression, pattern: pattern, not: true}
}
func LikeEscaped(column, pattern string) Predicate {
	return likePredicate{column: column, pattern: pattern, escaped: true}
}
func NotLikeEscaped(column, pattern string) Predicate {
	return likePredicate{column: column, pattern: pattern, not: true, escaped: true}
}
func LikeValueEscaped(expression Expression, pattern string) Predicate {
	return expressionLikePredicate{expression: expression, pattern: pattern, escaped: true}
}
func NotLikeValueEscaped(expression Expression, pattern string) Predicate {
	return expressionLikePredicate{expression: expression, pattern: pattern, not: true, escaped: true}
}

type expressionLikePredicate struct {
	expression Expression
	pattern    string
	not        bool
	escaped    bool
}

func (p expressionLikePredicate) renderPredicate(context *renderContext) (string, error) {
	if p.expression == nil {
		return "", fmt.Errorf("SQL like expression is required")
	}
	expression, err := p.expression.renderExpression(context)
	if err != nil {
		return "", err
	}
	operator := " LIKE "
	if p.not {
		operator = " NOT LIKE "
	}
	prepared := expression + operator + context.argument(p.pattern)
	if p.escaped {
		prepared += " ESCAPE '~'"
	}
	return prepared, nil
}
func (p likePredicate) renderPredicate(context *renderContext) (string, error) {
	if strings.TrimSpace(p.column) == "" {
		return "", fmt.Errorf("SQL like predicate column is required")
	}
	operator := " LIKE "
	if p.not {
		operator = " NOT LIKE "
	}
	prepared := context.renderer.Identifier(p.column) + operator + context.argument(p.pattern)
	if p.escaped {
		prepared += " ESCAPE '~'"
	}
	return prepared, nil
}

type setPredicate struct {
	column string
	values []any
	not    bool
}

func In(column string, values ...any) Predicate { return setPredicate{column: column, values: values} }
func NotIn(column string, values ...any) Predicate {
	return setPredicate{column: column, values: values, not: true}
}

type expressionSetPredicate struct {
	expression Expression
	values     []any
	not        bool
}

func InExpression(expression Expression, values ...any) Predicate {
	return expressionSetPredicate{expression: expression, values: values}
}
func NotInExpression(expression Expression, values ...any) Predicate {
	return expressionSetPredicate{expression: expression, values: values, not: true}
}
func (p expressionSetPredicate) renderPredicate(context *renderContext) (string, error) {
	if p.expression == nil || len(p.values) == 0 {
		return "", fmt.Errorf("SQL expression set predicate requires expression and values")
	}
	expression, err := p.expression.renderExpression(context)
	if err != nil {
		return "", err
	}
	placeholders := make([]string, len(p.values))
	for index, value := range p.values {
		placeholders[index] = context.argument(value)
	}
	operator := " IN "
	if p.not {
		operator = " NOT IN "
	}
	return expression + operator + "(" + strings.Join(placeholders, ", ") + ")", nil
}

func (p setPredicate) renderPredicate(context *renderContext) (string, error) {
	if strings.TrimSpace(p.column) == "" || len(p.values) == 0 {
		return "", fmt.Errorf("SQL set predicate requires a column and values")
	}
	placeholders := make([]string, len(p.values))
	for index, value := range p.values {
		placeholders[index] = context.argument(value)
	}
	operator := " IN "
	if p.not {
		operator = " NOT IN "
	}
	return context.renderer.Identifier(p.column) + operator + "(" + strings.Join(placeholders, ", ") + ")", nil
}

type nullPredicate struct {
	column     string
	expression Expression
	not        bool
}

func IsNull(column string) Predicate    { return nullPredicate{column: column} }
func IsNotNull(column string) Predicate { return nullPredicate{column: column, not: true} }
func IsNullExpression(expression Expression) Predicate {
	return nullPredicate{expression: expression}
}
func IsNotNullExpression(expression Expression) Predicate {
	return nullPredicate{expression: expression, not: true}
}

func (p nullPredicate) renderPredicate(context *renderContext) (string, error) {
	operand := ""
	if p.expression != nil {
		prepared, err := p.expression.renderExpression(context)
		if err != nil {
			return "", err
		}
		operand = prepared
	} else if strings.TrimSpace(p.column) != "" {
		operand = context.renderer.Identifier(p.column)
	} else {
		return "", fmt.Errorf("SQL null predicate column is required")
	}
	operator := " IS NULL"
	if p.not {
		operator = " IS NOT NULL"
	}
	return operand + operator, nil
}

type constantPredicate bool

func AlwaysFalse() Predicate { return constantPredicate(false) }
func AlwaysTrue() Predicate  { return constantPredicate(true) }

func (p constantPredicate) renderPredicate(*renderContext) (string, error) {
	if p {
		return "1 = 1", nil
	}
	return "1 = 0", nil
}

type notPredicate struct{ predicate Predicate }

func Not(predicate Predicate) Predicate { return notPredicate{predicate: predicate} }

func (p notPredicate) renderPredicate(context *renderContext) (string, error) {
	if p.predicate == nil {
		return "", fmt.Errorf("SQL NOT predicate requires child")
	}
	prepared, err := p.predicate.renderPredicate(context)
	if err != nil {
		return "", err
	}
	return "NOT (" + prepared + ")", nil
}

type compoundPredicate struct {
	operator   string
	predicates []Predicate
}

func And(predicates ...Predicate) Predicate {
	return compoundPredicate{operator: "AND", predicates: predicates}
}
func Or(predicates ...Predicate) Predicate {
	return compoundPredicate{operator: "OR", predicates: predicates}
}

func (p compoundPredicate) renderPredicate(context *renderContext) (string, error) {
	if len(p.predicates) == 0 {
		return "", fmt.Errorf("SQL compound predicate requires children")
	}
	parts := make([]string, len(p.predicates))
	for index, predicate := range p.predicates {
		if predicate == nil {
			return "", fmt.Errorf("SQL compound predicate child is required")
		}
		part, err := predicate.renderPredicate(context)
		if err != nil {
			return "", err
		}
		parts[index] = part
	}
	return "(" + strings.Join(parts, " "+p.operator+" ") + ")", nil
}
