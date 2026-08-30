package query

import (
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

// Renderer is the portable SQL surface required by the builders. A bound
// dialect renderer owns identifier quoting, placeholder syntax, and physical
// table qualification. Dialect identity is an optional capability so existing
// host renderers remain source-compatible with the portable builder surface.
type Renderer interface {
	Identifier(string) string
	Table(string) string
	Placeholder(int) string
}

type namedRenderer interface {
	Name() dialect.Name
}

type renderContext struct {
	renderer Renderer
	args     []any
	offset   int
}

func (c *renderContext) argument(value any) string {
	c.args = append(c.args, value)
	return c.renderer.Placeholder(c.offset + len(c.args))
}

func (c *renderContext) dialect() (dialect.Name, bool) {
	renderer, ok := c.renderer.(namedRenderer)
	if !ok {
		return "", false
	}
	return renderer.Name(), true
}

// PreparePredicate compiles a predicate fragment and its bound arguments.
// offset is the number of arguments already owned by the surrounding SQL.
func PreparePredicate(renderer Renderer, predicate Predicate, offset int) (string, []any, error) {
	if renderer == nil || predicate == nil || offset < 0 {
		return "", nil, fmt.Errorf("SQL predicate requires renderer, predicate, and non-negative offset")
	}
	context := &renderContext{renderer: renderer, offset: offset}
	prepared, err := predicate.renderPredicate(context)
	if err != nil {
		return "", nil, err
	}
	return prepared, append([]any(nil), context.args...), nil
}

// PrepareOrderBy compiles an ORDER BY fragment without accepting raw SQL.
func PrepareOrderBy(renderer Renderer, orders ...Order) (string, error) {
	if renderer == nil || len(orders) == 0 {
		return "", fmt.Errorf("SQL order requires renderer and terms")
	}
	context := &renderContext{renderer: renderer}
	parts := make([]string, len(orders))
	for index, order := range orders {
		prepared, err := order.render(context)
		if err != nil {
			return "", err
		}
		parts[index] = prepared
	}
	return "ORDER BY " + strings.Join(parts, ", "), nil
}
