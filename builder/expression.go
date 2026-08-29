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
func Sum(expression Expression) Expression {
	return functionExpression{name: "SUM", arguments: []Expression{expression}}
}
func Coalesce(expressions ...Expression) Expression {
	return functionExpression{name: "COALESCE", arguments: expressions}
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
