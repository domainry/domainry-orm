package builder

import (
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
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

type tableColumnExpression struct {
	table  string
	column string
}

// TableColumn qualifies a column with the renderer's physical table name,
// including any configured schema or prefix.
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

type insertedValueExpression struct{ column string }

// InsertedValue references the incoming row inside an upsert update clause.
// MySQL renders VALUES(column); PostgreSQL and SQLite render excluded.column.
// The column remains a validated identifier and never accepts raw SQL.
func InsertedValue(column string) Expression {
	return insertedValueExpression{column: strings.TrimSpace(column)}
}

func (e insertedValueExpression) renderExpression(context *renderContext) (string, error) {
	if e.column == "" {
		return "", fmt.Errorf("SQL inserted value column is required")
	}
	name, ok := context.dialect()
	if !ok {
		return "", fmt.Errorf("SQL inserted value requires a named dialect renderer")
	}
	switch name {
	case dialect.MySQL:
		return "VALUES(" + context.renderer.Identifier(e.column) + ")", nil
	case dialect.Postgres, dialect.SQLite:
		return context.renderer.Identifier("excluded") + "." + context.renderer.Identifier(e.column), nil
	default:
		return "", fmt.Errorf("SQL inserted value does not support dialect %q", name)
	}
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
func Sum(expression Expression) Expression {
	return functionExpression{name: "SUM", arguments: []Expression{expression}}
}
func Coalesce(expressions ...Expression) Expression {
	return functionExpression{name: "COALESCE", arguments: expressions}
}
func Max(expression Expression) Expression {
	return functionExpression{name: "MAX", arguments: []Expression{expression}}
}
func Min(expression Expression) Expression {
	return functionExpression{name: "MIN", arguments: []Expression{expression}}
}
func Lower(expression Expression) Expression {
	return functionExpression{name: "LOWER", arguments: []Expression{expression}}
}

// CountAll renders COUNT(*).
func CountAll() Expression { return functionExpression{name: "COUNT", arguments: []Expression{Star()}} }

// AllColumns is the compatibility name for an unqualified star expression.
func AllColumns() Expression { return Star() }

// -----------------------------------------------------------------------------
// CASE expression
// -----------------------------------------------------------------------------

type caseBranch struct {
	when Predicate
	then any
}

type caseExpression struct {
	branches   []caseBranch
	hasElse    bool
	elseResult any
}

// CaseWhen starts a searched CASE expression with its first WHEN branch.
func CaseWhen(when Predicate, then any) *caseExpression {
	return &caseExpression{branches: []caseBranch{{when: when, then: then}}}
}

// When appends another WHEN branch.
func (e *caseExpression) When(when Predicate, then any) *caseExpression {
	e.branches = append(e.branches, caseBranch{when: when, then: then})
	return e
}

// Else sets the ELSE result and terminates the builder chain.
func (e *caseExpression) Else(result any) *caseExpression {
	e.hasElse = true
	e.elseResult = result
	return e
}

func (e *caseExpression) renderExpression(context *renderContext) (string, error) {
	if len(e.branches) == 0 {
		return "", fmt.Errorf("SQL case expression requires at least one WHEN branch")
	}
	statement := "CASE"
	for _, branch := range e.branches {
		if branch.when == nil {
			return "", fmt.Errorf("SQL case WHEN predicate is required")
		}
		when, err := branch.when.renderPredicate(context)
		if err != nil {
			return "", err
		}
		then, err := caseResult(context, branch.then)
		if err != nil {
			return "", err
		}
		statement += " WHEN " + when + " THEN " + then
	}
	if e.hasElse {
		result, err := caseResult(context, e.elseResult)
		if err != nil {
			return "", err
		}
		statement += " ELSE " + result
	}
	return statement + " END", nil
}

// caseResult renders a CASE THEN/ELSE result. Expression values are rendered
// inline; any other value is bound as a placeholder argument.
func caseResult(context *renderContext, value any) (string, error) {
	if expression, ok := value.(Expression); ok {
		return expression.renderExpression(context)
	}
	return context.argument(value), nil
}

func (e functionExpression) renderExpression(context *renderContext) (string, error) {
	if !safeFunctionName(e.name) {
		return "", fmt.Errorf("SQL function expression requires a name")
	}
	if len(e.arguments) == 0 {
		// Zero-argument functions such as ROW_NUMBER() render with empty parens.
		return e.name + "()", nil
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

func safeFunctionName(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if character == '_' || character >= 'A' && character <= 'Z' || index > 0 && character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}
