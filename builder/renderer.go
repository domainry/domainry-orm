package builder

import "github.com/domainry/domainry-orm/dialect"

// Renderer is the portable SQL surface required by the builders. A bound
// dialect renderer owns identifier quoting, placeholder syntax, and physical
// table qualification. Name reports the active dialect so builders can emit
// dialect-specific syntax (row locking, ILIKE, upsert, NULLS ordering, ...).
type Renderer interface {
	Name() dialect.Name
	Identifier(string) string
	Table(string) string
	Placeholder(int) string
}

type renderContext struct {
	renderer Renderer
	args     []any
}

func (c *renderContext) argument(value any) string {
	c.args = append(c.args, value)
	return c.renderer.Placeholder(len(c.args))
}

func (c *renderContext) dialect() dialect.Name { return c.renderer.Name() }
