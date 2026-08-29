package builder

import (
	"fmt"
	"strings"
)

type DeleteBuilder struct {
	renderer  Renderer
	table     string
	predicate Predicate
	returning []string
}

func NewDeleteBuilder(renderer Renderer, table string) *DeleteBuilder {
	return &DeleteBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}
func (b *DeleteBuilder) Where(predicate Predicate) *DeleteBuilder { b.predicate = predicate; return b }

// Returning appends a RETURNING clause (PostgreSQL / SQLite).
func (b *DeleteBuilder) Returning(columns ...string) *DeleteBuilder {
	b.returning = append([]string(nil), columns...)
	return b
}

func (b *DeleteBuilder) Build() (string, []any, error) {
	if b == nil || b.renderer == nil || b.table == "" {
		return "", nil, fmt.Errorf("SQL delete requires renderer and table")
	}
	if b.predicate == nil {
		return "", nil, fmt.Errorf("SQL delete requires an explicit predicate")
	}
	context := &renderContext{renderer: b.renderer}
	where, err := b.predicate.renderPredicate(context)
	if err != nil {
		return "", nil, err
	}
	statement := "DELETE FROM " + b.renderer.Table(b.table) + " WHERE " + where
	if returning := renderReturning(b.renderer, b.returning); returning != "" {
		statement += returning
	}
	return statement, append([]any(nil), context.args...), nil
}
