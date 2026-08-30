package query

import "strings"

// Assignment is a "column = expression" pair used by UPDATE SET and upsert
// DO UPDATE / ON DUPLICATE KEY UPDATE clauses.
type Assignment struct {
	column     string
	expression Expression
}

// Assign sets a column to a literal value.
func Assign(column string, value any) Assignment {
	return Assignment{column: strings.TrimSpace(column), expression: Value(value)}
}

// AssignExpression sets a column to an arbitrary expression.
func AssignExpression(column string, expression Expression) Assignment {
	return Assignment{column: strings.TrimSpace(column), expression: expression}
}

func (a Assignment) render(context *renderContext) (string, error) {
	if a.column == "" || a.expression == nil {
		return "", errAssignment
	}
	expression, err := a.expression.renderExpression(context)
	if err != nil {
		return "", err
	}
	return context.renderer.Identifier(a.column) + " = " + expression, nil
}

func renderAssignments(context *renderContext, assignments []Assignment) (string, error) {
	parts := make([]string, len(assignments))
	for index, assignment := range assignments {
		rendered, err := assignment.render(context)
		if err != nil {
			return "", err
		}
		parts[index] = rendered
	}
	return strings.Join(parts, ", "), nil
}
