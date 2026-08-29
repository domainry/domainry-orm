package builder

import (
	"fmt"
	"strings"
)

var errAssignment = fmt.Errorf("SQL assignment is invalid")

type UpdateBuilder struct {
	renderer    Renderer
	table       string
	assignments []Assignment
	predicate   Predicate
	returning   []string
}

func NewUpdateBuilder(renderer Renderer, table string) *UpdateBuilder {
	return &UpdateBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}
func (b *UpdateBuilder) Set(column string, value any) *UpdateBuilder {
	return b.SetExpression(column, Value(value))
}
func (b *UpdateBuilder) SetExpression(column string, expression Expression) *UpdateBuilder {
	b.assignments = append(b.assignments, AssignExpression(column, expression))
	return b
}
func (b *UpdateBuilder) Where(predicate Predicate) *UpdateBuilder { b.predicate = predicate; return b }

// Returning appends a RETURNING clause (PostgreSQL / SQLite).
func (b *UpdateBuilder) Returning(columns ...string) *UpdateBuilder {
	b.returning = append([]string(nil), columns...)
	return b
}

func (b *UpdateBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" || len(b.assignments) == 0 {
		return "", nil, fmt.Errorf("SQL update requires renderer, table, and assignments")
	}
	if b.predicate == nil {
		return "", nil, fmt.Errorf("SQL update requires an explicit predicate")
	}
	context := &renderContext{renderer: b.renderer}
	assignments, err := renderAssignments(context, b.assignments)
	if err != nil {
		return "", nil, err
	}
	where, err := b.predicate.renderPredicate(context)
	if err != nil {
		return "", nil, err
	}
	statement := "UPDATE " + b.renderer.Table(b.table) + " SET " + assignments + " WHERE " + where
	if returning := renderReturning(b.renderer, b.returning); returning != "" {
		statement += returning
	}
	return statement, append([]any(nil), context.args...), nil
}

// renderReturning renders a shared RETURNING clause for INSERT/UPDATE/DELETE.
func renderReturning(renderer Renderer, columns []string) string {
	if len(columns) == 0 {
		return ""
	}
	quoted := make([]string, len(columns))
	for index, column := range columns {
		if strings.TrimSpace(column) == "*" {
			quoted[index] = "*"
			continue
		}
		quoted[index] = renderer.Identifier(column)
	}
	return " RETURNING " + strings.Join(quoted, ", ")
}
