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
