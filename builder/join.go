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
	subquery  *SelectBuilder
	lateral   bool
}

func InnerJoin(table, alias string, predicate Predicate) Join {
	return Join{kind: "INNER JOIN", table: table, alias: alias, predicate: predicate}
}
func LeftJoin(table, alias string, predicate Predicate) Join {
	return Join{kind: "LEFT JOIN", table: table, alias: alias, predicate: predicate}
}
func RightJoin(table, alias string, predicate Predicate) Join {
	return Join{kind: "RIGHT JOIN", table: table, alias: alias, predicate: predicate}
}
func FullJoin(table, alias string, predicate Predicate) Join {
	return Join{kind: "FULL JOIN", table: table, alias: alias, predicate: predicate}
}

// CrossJoin joins without an ON predicate.
func CrossJoin(table, alias string) Join {
	return Join{kind: "CROSS JOIN", table: table, alias: alias}
}

// InnerJoinSubquery, LeftJoinSubquery, RightJoinSubquery and FullJoinSubquery
// join against a derived table. Use Lateral on the returned Join for LATERAL.
func InnerJoinSubquery(subquery *SelectBuilder, alias string, predicate Predicate) Join {
	return Join{kind: "INNER JOIN", subquery: subquery, alias: alias, predicate: predicate}
}
func LeftJoinSubquery(subquery *SelectBuilder, alias string, predicate Predicate) Join {
	return Join{kind: "LEFT JOIN", subquery: subquery, alias: alias, predicate: predicate}
}
func RightJoinSubquery(subquery *SelectBuilder, alias string, predicate Predicate) Join {
	return Join{kind: "RIGHT JOIN", subquery: subquery, alias: alias, predicate: predicate}
}
func FullJoinSubquery(subquery *SelectBuilder, alias string, predicate Predicate) Join {
	return Join{kind: "FULL JOIN", subquery: subquery, alias: alias, predicate: predicate}
}

// Lateral marks the join source as LATERAL (only valid for subquery joins).
func (j Join) Lateral() Join { j.lateral = true; return j }

func (j Join) render(context *renderContext) (string, error) {
	cross := j.kind == "CROSS JOIN"
	if j.subquery == nil && strings.TrimSpace(j.table) == "" {
		return "", fmt.Errorf("SQL join requires table or subquery")
	}
	if !cross && j.predicate == nil {
		return "", fmt.Errorf("SQL join requires a predicate")
	}

	var source string
	if j.subquery != nil {
		if strings.TrimSpace(j.alias) == "" {
			return "", fmt.Errorf("SQL join subquery requires an alias")
		}
		inner, err := j.subquery.render(context, false)
		if err != nil {
			return "", err
		}
		prefix := ""
		if j.lateral {
			prefix = "LATERAL "
		}
		source = prefix + "(" + inner + ") AS " + context.renderer.Identifier(j.alias)
	} else {
		source = context.renderer.Table(j.table)
		if strings.TrimSpace(j.alias) != "" {
			source += " AS " + context.renderer.Identifier(j.alias)
		}
	}

	value := j.kind + " " + source
	if cross {
		return value, nil
	}
	predicate, err := j.predicate.renderPredicate(context)
	if err != nil {
		return "", err
	}
	return value + " ON " + predicate, nil
}
