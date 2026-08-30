package query

import (
	"fmt"
	"strings"
)

type Projection struct {
	expression Expression
	alias      string
}

func Project(expression Expression) Projection { return Projection{expression: expression} }
func ProjectAs(expression Expression, alias string) Projection {
	return Projection{expression: expression, alias: alias}
}

func (p Projection) render(context *renderContext) (string, error) {
	if p.expression == nil {
		return "", fmt.Errorf("SQL projection expression is required")
	}
	value, err := p.expression.renderExpression(context)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(p.alias) != "" {
		value += " AS " + context.renderer.Identifier(p.alias)
	}
	return value, nil
}
