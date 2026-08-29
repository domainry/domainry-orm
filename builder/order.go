package builder

import (
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

type nullsPlacement int

const (
	nullsDefault nullsPlacement = iota
	nullsFirst
	nullsLast
)

type Order struct {
	column     string
	expression Expression
	direction  string
	nulls      nullsPlacement
}

func Ascending(column string) Order  { return Order{column: column, direction: "ASC"} }
func Descending(column string) Order { return Order{column: column, direction: "DESC"} }
func AscendingExpression(expression Expression) Order {
	return Order{expression: expression, direction: "ASC"}
}
func DescendingExpression(expression Expression) Order {
	return Order{expression: expression, direction: "DESC"}
}

// NullsFirst and NullsLast control NULL ordering. PostgreSQL and SQLite emit
// native NULLS FIRST/LAST; MySQL lacks the syntax and is emulated with a
// leading ISNULL sort key.
func (o Order) NullsFirst() Order { o.nulls = nullsFirst; return o }
func (o Order) NullsLast() Order  { o.nulls = nullsLast; return o }

func (o Order) render(context *renderContext) (string, error) {
	operand, err := o.operand(context)
	if err != nil {
		return "", err
	}
	term := operand + " " + o.direction
	if o.nulls == nullsDefault {
		return term, nil
	}
	if name, _ := context.dialect(); name == dialect.MySQL {
		// MySQL sorts NULLs first for ASC and last for DESC by default. Emulate
		// explicit placement with a leading ISNULL key.
		flag := "ISNULL(" + operand + ")"
		if o.nulls == nullsLast {
			return flag + " ASC, " + term, nil
		}
		return flag + " DESC, " + term, nil
	}
	if o.nulls == nullsFirst {
		return term + " NULLS FIRST", nil
	}
	return term + " NULLS LAST", nil
}

func (o Order) operand(context *renderContext) (string, error) {
	if o.expression != nil {
		return o.expression.renderExpression(context)
	}
	if strings.TrimSpace(o.column) == "" {
		return "", fmt.Errorf("SQL order column or expression is required")
	}
	return context.renderer.Identifier(o.column), nil
}
