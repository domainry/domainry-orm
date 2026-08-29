package builder

import (
	"fmt"
	"strings"
)

type DeleteBuilder struct {
	renderer  Renderer
	table     string
	predicate Predicate
}

func NewDeleteBuilder(renderer Renderer, table string) *DeleteBuilder {
	return &DeleteBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}
func (b *DeleteBuilder) Where(predicate Predicate) *DeleteBuilder { b.predicate = predicate; return b }

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
	return "DELETE FROM " + b.renderer.Table(b.table) + " WHERE " + where, append([]any(nil), context.args...), nil
}
