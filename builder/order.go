package builder

import (
	"fmt"
	"strings"
)

type Order struct {
	column     string
	expression Expression
	direction  string
}

func Ascending(column string) Order  { return Order{column: column, direction: "ASC"} }
func Descending(column string) Order { return Order{column: column, direction: "DESC"} }
func AscendingExpression(expression Expression) Order {
	return Order{expression: expression, direction: "ASC"}
}
func DescendingExpression(expression Expression) Order {
	return Order{expression: expression, direction: "DESC"}
}

func (o Order) render(context *renderContext) (string, error) {
	if o.expression != nil {
		value, err := o.expression.renderExpression(context)
		if err != nil {
			return "", err
		}
		return value + " " + o.direction, nil
	}
	if strings.TrimSpace(o.column) == "" {
		return "", fmt.Errorf("SQL order column or expression is required")
	}
	return context.renderer.Identifier(o.column) + " " + o.direction, nil
}
