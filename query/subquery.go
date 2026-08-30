package query

import (
	"fmt"
	"strings"
)

type scalarSubqueryExpression struct {
	table      string
	projection Expression
	predicate  Predicate
}

func ScalarSubquery(table string, projection Expression, predicate Predicate) Expression {
	return scalarSubqueryExpression{table: strings.TrimSpace(table), projection: projection, predicate: predicate}
}

func (e scalarSubqueryExpression) renderExpression(context *renderContext) (string, error) {
	if e.table == "" || e.projection == nil {
		return "", fmt.Errorf("SQL scalar subquery requires table and projection")
	}
	projection, err := e.projection.renderExpression(context)
	if err != nil {
		return "", err
	}
	statement := "SELECT " + projection + " FROM " + context.renderer.Table(e.table)
	if e.predicate != nil {
		where, err := e.predicate.renderPredicate(context)
		if err != nil {
			return "", err
		}
		statement += " WHERE " + where
	}
	return "(" + statement + ")", nil
}

type existsPredicate struct {
	table     string
	predicate Predicate
	not       bool
}

func Exists(table string, predicate Predicate) Predicate {
	return existsPredicate{table: strings.TrimSpace(table), predicate: predicate}
}
func NotExists(table string, predicate Predicate) Predicate {
	return existsPredicate{table: strings.TrimSpace(table), predicate: predicate, not: true}
}
func (p existsPredicate) renderPredicate(context *renderContext) (string, error) {
	if p.table == "" || p.predicate == nil {
		return "", fmt.Errorf("SQL exists predicate requires table and predicate")
	}
	where, err := p.predicate.renderPredicate(context)
	if err != nil {
		return "", err
	}
	keyword := "EXISTS"
	if p.not {
		keyword = "NOT EXISTS"
	}
	return keyword + " (SELECT 1 FROM " + context.renderer.Table(p.table) + " WHERE " + where + ")", nil
}
