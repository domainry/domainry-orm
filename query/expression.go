package query

import (
	"fmt"
	"regexp"
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

type concatExpression struct{ expressions []Expression }

// Concat joins text expressions using the active dialect's native syntax.
// Values must be supplied with Value so they remain bound parameters.
func Concat(expressions ...Expression) Expression {
	return concatExpression{expressions: append([]Expression(nil), expressions...)}
}

func (e concatExpression) renderExpression(context *renderContext) (string, error) {
	if len(e.expressions) < 2 {
		return "", fmt.Errorf("SQL concat expression requires at least two operands")
	}
	rendered := make([]string, len(e.expressions))
	for index, expression := range e.expressions {
		if expression == nil {
			return "", fmt.Errorf("SQL concat expression operand is required")
		}
		value, err := expression.renderExpression(context)
		if err != nil {
			return "", err
		}
		rendered[index] = value
	}
	name, ok := context.dialect()
	if !ok {
		return "", fmt.Errorf("SQL concat expression requires a named dialect renderer")
	}
	if name == dialect.MySQL {
		return "CONCAT(" + strings.Join(rendered, ", ") + ")", nil
	}
	if name == dialect.Postgres || name == dialect.SQLite {
		return "(" + strings.Join(rendered, " || ") + ")", nil
	}
	return "", fmt.Errorf("SQL concat expression does not support dialect %q", name)
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
