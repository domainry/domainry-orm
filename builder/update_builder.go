package builder

import (
	"fmt"
	"strings"
)

type assignment struct {
	column     string
	expression Expression
}

type UpdateBuilder struct {
	renderer    Renderer
	table       string
	assignments []assignment
	predicate   Predicate
}

func NewUpdateBuilder(renderer Renderer, table string) *UpdateBuilder {
	return &UpdateBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}
func (b *UpdateBuilder) Set(column string, value any) *UpdateBuilder {
	return b.SetExpression(column, Value(value))
}
func (b *UpdateBuilder) SetExpression(column string, expression Expression) *UpdateBuilder {
	b.assignments = append(b.assignments, assignment{column: column, expression: expression})
	return b
}
func (b *UpdateBuilder) Where(predicate Predicate) *UpdateBuilder { b.predicate = predicate; return b }

func (b *UpdateBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" || len(b.assignments) == 0 {
		return "", nil, fmt.Errorf("SQL update requires renderer, table, and assignments")
	}
	if b.predicate == nil {
		return "", nil, fmt.Errorf("SQL update requires an explicit predicate")
	}
	context := &renderContext{renderer: b.renderer}
	assignments := make([]string, len(b.assignments))
	for index, assignment := range b.assignments {
		if strings.TrimSpace(assignment.column) == "" || assignment.expression == nil {
			return "", nil, fmt.Errorf("SQL update assignment is invalid")
		}
		expression, err := assignment.expression.renderExpression(context)
		if err != nil {
			return "", nil, err
		}
		assignments[index] = b.renderer.Identifier(assignment.column) + " = " + expression
	}
	where, err := b.predicate.renderPredicate(context)
	if err != nil {
		return "", nil, err
	}
	statement := "UPDATE " + b.renderer.Table(b.table) + " SET " + strings.Join(assignments, ", ") + " WHERE " + where
	return statement, append([]any(nil), context.args...), nil
}
