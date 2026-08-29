package builder

import "github.com/domainry/domainry-orm/dialect"

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
}

func (c *renderContext) argument(value any) string {
	c.args = append(c.args, value)
	return c.renderer.Placeholder(len(c.args))
}

func (c *renderContext) dialect() (dialect.Name, bool) {
	renderer, ok := c.renderer.(namedRenderer)
	if !ok {
		return "", false
	}
	return renderer.Name(), true
}
