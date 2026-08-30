package schema

import "github.com/domainry/domainry-orm/dialect"

// Renderer is the portable SQL surface required by schema declarations.
type Renderer interface {
	Identifier(string) string
	Table(string) string
	Placeholder(int) string
}

type namedRenderer interface {
	Name() dialect.Name
}
