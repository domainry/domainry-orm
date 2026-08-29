package builder

// Renderer is the portable SQL surface required by the builders. A bound
// dialect renderer owns identifier quoting, placeholder syntax, and physical
// table qualification.
type Renderer interface {
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
