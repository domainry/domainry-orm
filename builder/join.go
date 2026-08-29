package builder

import (
	"fmt"
	"strings"
)

type Join struct {
	kind      string
	table     string
	alias     string
	predicate Predicate
}

func InnerJoin(table, alias string, predicate Predicate) Join {
	return Join{kind: "INNER JOIN", table: table, alias: alias, predicate: predicate}
}
func LeftJoin(table, alias string, predicate Predicate) Join {
	return Join{kind: "LEFT JOIN", table: table, alias: alias, predicate: predicate}
}

func (j Join) render(context *renderContext) (string, error) {
	if strings.TrimSpace(j.table) == "" || j.predicate == nil {
		return "", fmt.Errorf("SQL join requires table and predicate")
	}
	value := j.kind + " " + context.renderer.Table(j.table)
	if strings.TrimSpace(j.alias) != "" {
		value += " AS " + context.renderer.Identifier(j.alias)
	}
	predicate, err := j.predicate.renderPredicate(context)
	if err != nil {
		return "", err
	}
	return value + " ON " + predicate, nil
}
