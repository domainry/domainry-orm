package builder

import (
	"fmt"
	"strings"
)

type Expression interface {
	renderExpression(*renderContext) (string, error)
}

type columnExpression struct{ column string }

func Column(column string) Expression { return columnExpression{column: column} }

func (e columnExpression) renderExpression(context *renderContext) (string, error) {
	if strings.TrimSpace(e.column) == "" {
		return "", fmt.Errorf("SQL expression column is required")
	}
	return context.renderer.Identifier(e.column), nil
}

type qualifiedColumnExpression struct {
	qualifier string
	column    string
}

func QualifiedColumn(qualifier, column string) Expression {
	return qualifiedColumnExpression{qualifier: qualifier, column: column}
}

type tableColumnExpression struct{ table, column string }

// TableColumn qualifies a column with the renderer's physical table name,
// including configured schema and prefix.
func TableColumn(table, column string) Expression {
	return tableColumnExpression{table: table, column: column}
}
func (e tableColumnExpression) renderExpression(context *renderContext) (string, error) {
	if strings.TrimSpace(e.table) == "" || strings.TrimSpace(e.column) == "" {
		return "", fmt.Errorf("SQL table column requires table and column")
	}
	return context.renderer.Table(e.table) + "." + context.renderer.Identifier(e.column), nil
}

func (e qualifiedColumnExpression) renderExpression(context *renderContext) (string, error) {
	if strings.TrimSpace(e.qualifier) == "" || strings.TrimSpace(e.column) == "" {
		return "", fmt.Errorf("SQL qualified column requires qualifier and column")
	}
	return context.renderer.Identifier(e.qualifier) + "." + context.renderer.Identifier(e.column), nil
}

type valueExpression struct{ value any }

func Value(value any) Expression { return valueExpression{value: value} }

func (e valueExpression) renderExpression(context *renderContext) (string, error) {
	return context.argument(e.value), nil
}

type arithmeticExpression struct {
	left     Expression
	operator string
	right    Expression
}

func Add(left, right Expression) Expression {
	return arithmeticExpression{left: left, operator: "+", right: right}
}
func Subtract(left, right Expression) Expression {
	return arithmeticExpression{left: left, operator: "-", right: right}
}

func (e arithmeticExpression) renderExpression(context *renderContext) (string, error) {
	if e.left == nil || e.right == nil {
		return "", fmt.Errorf("SQL arithmetic expression operands are required")
	}
	left, err := e.left.renderExpression(context)
	if err != nil {
		return "", err
	}
	right, err := e.right.renderExpression(context)
	if err != nil {
		return "", err
	}
	return left + " " + e.operator + " " + right, nil
}

type functionExpression struct {
	name      string
	arguments []Expression
}

func Count(expression Expression) Expression {
	return functionExpression{name: "COUNT", arguments: []Expression{expression}}
}
func CountAll() Expression {
	return functionExpression{name: "COUNT", arguments: []Expression{keywordExpression{keyword: "*"}}}
}
func Sum(expression Expression) Expression {
	return functionExpression{name: "SUM", arguments: []Expression{expression}}
}
func Max(expression Expression) Expression {
	return functionExpression{name: "MAX", arguments: []Expression{expression}}
}
func Min(expression Expression) Expression {
	return functionExpression{name: "MIN", arguments: []Expression{expression}}
}
func Coalesce(expressions ...Expression) Expression {
	return functionExpression{name: "COALESCE", arguments: expressions}
}
func Lower(expression Expression) Expression {
	return functionExpression{name: "LOWER", arguments: []Expression{expression}}
}

type keywordExpression struct{ keyword string }

func AllColumns() Expression { return keywordExpression{keyword: "*"} }

func (e keywordExpression) renderExpression(*renderContext) (string, error) {
	if e.keyword != "*" {
		return "", fmt.Errorf("unsupported SQL keyword expression")
	}
	return e.keyword, nil
}

type caseBranch struct {
	predicate Predicate
	value     Expression
}
type caseExpression struct {
	branches  []caseBranch
	otherwise Expression
}

func CaseWhen(predicate Predicate, value any) *caseExpression {
	return &caseExpression{branches: []caseBranch{{predicate: predicate, value: Value(value)}}}
}
func (e *caseExpression) When(predicate Predicate, value any) *caseExpression {
	e.branches = append(e.branches, caseBranch{predicate: predicate, value: Value(value)})
	return e
}
func (e *caseExpression) Else(value any) Expression {
	e.otherwise = Value(value)
	return e
}
func (e *caseExpression) renderExpression(context *renderContext) (string, error) {
	if e == nil || len(e.branches) == 0 || e.otherwise == nil {
		return "", fmt.Errorf("SQL case expression requires branches and fallback")
	}
	statement := "CASE"
	for _, branch := range e.branches {
		if branch.predicate == nil || branch.value == nil {
			return "", fmt.Errorf("SQL case branch is invalid")
		}
		condition, err := branch.predicate.renderPredicate(context)
		if err != nil {
			return "", err
		}
		value, err := branch.value.renderExpression(context)
		if err != nil {
			return "", err
		}
		statement += " WHEN " + condition + " THEN " + value
	}
	fallback, err := e.otherwise.renderExpression(context)
	if err != nil {
		return "", err
	}
	return statement + " ELSE " + fallback + " END", nil
}

func (e functionExpression) renderExpression(context *renderContext) (string, error) {
	if len(e.arguments) == 0 {
		return "", fmt.Errorf("SQL function expression requires arguments")
	}
	arguments := make([]string, len(e.arguments))
	for index, argument := range e.arguments {
		if argument == nil {
			return "", fmt.Errorf("SQL function argument is required")
		}
		rendered, err := argument.renderExpression(context)
		if err != nil {
			return "", err
		}
		arguments[index] = rendered
	}
	return e.name + "(" + strings.Join(arguments, ", ") + ")", nil
}
