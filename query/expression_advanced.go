package query

import (
	"fmt"
	"regexp"
	"strings"
)

// Func renders an arbitrary SQL function call, e.g. Func("ROUND", col, Value(2)).
func Func(name string, arguments ...Expression) Expression {
	return functionExpression{name: strings.ToUpper(strings.TrimSpace(name)), arguments: arguments}
}

// Avg, Abs, Upper, Length round out the common scalar/aggregate helpers.
func Avg(expression Expression) Expression    { return Func("AVG", expression) }
func Abs(expression Expression) Expression    { return Func("ABS", expression) }
func Upper(expression Expression) Expression  { return Func("UPPER", expression) }
func Length(expression Expression) Expression { return Func("LENGTH", expression) }

// Multiply and Divide extend the arithmetic operators.
func Multiply(left, right Expression) Expression {
	return arithmeticExpression{left: left, operator: "*", right: right}
}
func Divide(left, right Expression) Expression {
	return arithmeticExpression{left: left, operator: "/", right: right}
}

// -----------------------------------------------------------------------------
// Star
// -----------------------------------------------------------------------------

type starExpression struct{}

// Star renders "*" for use in projections such as Project(Star()).
func Star() Expression { return starExpression{} }

func (starExpression) renderExpression(*renderContext) (string, error) { return "*", nil }

// -----------------------------------------------------------------------------
// CAST
// -----------------------------------------------------------------------------

type castExpression struct {
	expression Expression
	typ        string
}

func Cast(expression Expression, typ string) Expression {
	return castExpression{expression: expression, typ: strings.TrimSpace(typ)}
}

func (e castExpression) renderExpression(context *renderContext) (string, error) {
	if e.expression == nil || !safeCastType(e.typ) {
		return "", fmt.Errorf("SQL cast requires an expression and type")
	}
	inner, err := e.expression.renderExpression(context)
	if err != nil {
		return "", err
	}
	return "CAST(" + inner + " AS " + e.typ + ")", nil
}

func safeCastType(value string) bool {
	if value == "" {
		return false
	}
	depth := 0
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9', character == '_', character == ' ', character == ',':
		case character == '(':
			depth++
		case character == ')' && depth > 0:
			depth--
		default:
			return false
		}
	}
	return depth == 0
}

// -----------------------------------------------------------------------------
// Aggregate FILTER (WHERE ...)
// -----------------------------------------------------------------------------

type filteredExpression struct {
	aggregate Expression
	predicate Predicate
}

// Filtered attaches a FILTER (WHERE ...) clause to an aggregate expression.
func Filtered(aggregate Expression, predicate Predicate) Expression {
	return filteredExpression{aggregate: aggregate, predicate: predicate}
}

func (e filteredExpression) renderExpression(context *renderContext) (string, error) {
	if e.aggregate == nil || e.predicate == nil {
		return "", fmt.Errorf("SQL filtered aggregate requires an aggregate and predicate")
	}
	aggregate, err := e.aggregate.renderExpression(context)
	if err != nil {
		return "", err
	}
	where, err := e.predicate.renderPredicate(context)
	if err != nil {
		return "", err
	}
	return aggregate + " FILTER (WHERE " + where + ")", nil
}

// -----------------------------------------------------------------------------
// Window functions
// -----------------------------------------------------------------------------

// WindowSpec describes an OVER (...) clause.
type WindowSpec struct {
	partitions []Expression
	orders     []Order
	frame      string
}

func Window() WindowSpec { return WindowSpec{} }

func (w WindowSpec) PartitionBy(expressions ...Expression) WindowSpec {
	w.partitions = append(append([]Expression(nil), w.partitions...), expressions...)
	return w
}
func (w WindowSpec) OrderBy(orders ...Order) WindowSpec {
	w.orders = append(append([]Order(nil), w.orders...), orders...)
	return w
}

// Frame sets a raw frame clause such as "ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW".
func (w WindowSpec) Frame(frame string) WindowSpec {
	w.frame = strings.TrimSpace(frame)
	return w
}

func (w WindowSpec) render(context *renderContext) (string, error) {
	parts := make([]string, 0, 3)
	if len(w.partitions) > 0 {
		rendered, err := renderExpressionList(context, w.partitions)
		if err != nil {
			return "", err
		}
		parts = append(parts, "PARTITION BY "+rendered)
	}
	if len(w.orders) > 0 {
		orders := make([]string, len(w.orders))
		for index, order := range w.orders {
			rendered, err := order.render(context)
			if err != nil {
				return "", err
			}
			orders[index] = rendered
		}
		parts = append(parts, "ORDER BY "+strings.Join(orders, ", "))
	}
	if w.frame != "" {
		if !safeWindowFrame.MatchString(strings.ToUpper(w.frame)) {
			return "", fmt.Errorf("SQL window frame is invalid")
		}
		parts = append(parts, w.frame)
	}
	return strings.Join(parts, " "), nil
}

var safeWindowFrame = regexp.MustCompile(`^(ROWS|RANGE|GROUPS) (BETWEEN (UNBOUNDED PRECEDING|[0-9]+ PRECEDING|CURRENT ROW|[0-9]+ FOLLOWING|UNBOUNDED FOLLOWING) AND (UNBOUNDED PRECEDING|[0-9]+ PRECEDING|CURRENT ROW|[0-9]+ FOLLOWING|UNBOUNDED FOLLOWING)|(UNBOUNDED PRECEDING|[0-9]+ PRECEDING|CURRENT ROW|[0-9]+ FOLLOWING|UNBOUNDED FOLLOWING))$`)

type windowExpression struct {
	function Expression
	spec     WindowSpec
}

// Over attaches an OVER (...) window clause to a function expression.
func Over(function Expression, spec WindowSpec) Expression {
	return windowExpression{function: function, spec: spec}
}

func (e windowExpression) renderExpression(context *renderContext) (string, error) {
	if e.function == nil {
		return "", fmt.Errorf("SQL window expression requires a function")
	}
	function, err := e.function.renderExpression(context)
	if err != nil {
		return "", err
	}
	spec, err := e.spec.render(context)
	if err != nil {
		return "", err
	}
	return function + " OVER (" + spec + ")", nil
}

// Common window functions with no arguments.
func RowNumber() Expression { return functionExpression{name: "ROW_NUMBER"} }
func Rank() Expression      { return functionExpression{name: "RANK"} }
func DenseRank() Expression { return functionExpression{name: "DENSE_RANK"} }

// Lag and Lead reference neighbouring rows within a window.
func Lag(expression Expression) Expression  { return Func("LAG", expression) }
func Lead(expression Expression) Expression { return Func("LEAD", expression) }
