package builder

import (
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

type conflictAction int

const (
	conflictNone conflictAction = iota
	conflictDoNothing
	conflictDoUpdate
)

type onConflict struct {
	targets     []string
	action      conflictAction
	assignments []Assignment
	predicate   Predicate // optional WHERE on DO UPDATE
}

type InsertBuilder struct {
	renderer      Renderer
	table         string
	columns       []string
	rows          [][]any
	source        *SelectBuilder
	returning     []string
	conflict      *onConflict
	duplicate     []Assignment // MySQL ON DUPLICATE KEY UPDATE
	workspaceID   string
	workspaceMode bool
	buildError    error
}

func NewInsertBuilder(renderer Renderer, table string) *InsertBuilder {
	return &InsertBuilder{renderer: renderer, table: strings.TrimSpace(table)}
}
func (b *InsertBuilder) Columns(columns ...string) *InsertBuilder {
	b.columns = append([]string(nil), columns...)
	return b
}
func (b *InsertBuilder) Values(values ...any) *InsertBuilder {
	b.rows = append(b.rows, append([]any(nil), values...))
	return b
}

// FromSelect populates the INSERT from a SELECT query instead of VALUES.
func (b *InsertBuilder) FromSelect(source *SelectBuilder) *InsertBuilder {
	b.source = source
	return b
}

// Returning appends a RETURNING clause (PostgreSQL / SQLite).
func (b *InsertBuilder) Returning(columns ...string) *InsertBuilder {
	b.returning = append([]string(nil), columns...)
	return b
}

// OnConflictDoNothing skips duplicate rows across supported dialects. MySQL
// uses a no-op ON DUPLICATE KEY UPDATE so unrelated insert errors are not
// suppressed by INSERT IGNORE.
func (b *InsertBuilder) OnConflictDoNothing(targets ...string) *InsertBuilder {
	b.conflict = &onConflict{targets: targets, action: conflictDoNothing}
	return b
}

// OnConflictDoUpdate emits ON CONFLICT (targets) DO UPDATE SET assignments.
func (b *InsertBuilder) OnConflictDoUpdate(targets []string, assignments ...Assignment) *InsertBuilder {
	b.conflict = &onConflict{targets: targets, action: conflictDoUpdate, assignments: assignments}
	return b
}

// OnConflictDoUpdateWhere adds a WHERE clause to the DO UPDATE action.
func (b *InsertBuilder) OnConflictDoUpdateWhere(targets []string, predicate Predicate, assignments ...Assignment) *InsertBuilder {
	b.conflict = &onConflict{targets: targets, action: conflictDoUpdate, assignments: assignments, predicate: predicate}
	return b
}

// OnDuplicateKeyUpdate emits MySQL ON DUPLICATE KEY UPDATE assignments.
func (b *InsertBuilder) OnDuplicateKeyUpdate(assignments ...Assignment) *InsertBuilder {
	b.duplicate = append([]Assignment(nil), assignments...)
	return b
}

func (b *InsertBuilder) Build() (string, []any, error) {
	if b != nil && b.buildError != nil {
		return "", nil, b.buildError
	}
	if b == nil || b.renderer == nil || b.table == "" || len(b.columns) == 0 {
		return "", nil, fmt.Errorf("SQL insert requires renderer, table, and columns")
	}
	if len(b.rows) == 0 && b.source == nil {
		return "", nil, fmt.Errorf("SQL insert requires values or a source query")
	}
	if len(b.rows) > 0 && b.source != nil {
		return "", nil, fmt.Errorf("SQL insert cannot combine values and a source query")
	}
	if b.workspaceMode && b.source != nil {
		return "", nil, fmt.Errorf("SQL workspace insert does not support a SELECT source")
	}
	if b.workspaceMode {
		for _, column := range b.columns {
			if strings.EqualFold(strings.TrimSpace(column), WorkspaceIDColumn) {
				return "", nil, fmt.Errorf("SQL workspace insert owns the workspace_id column")
			}
		}
	}
	columnNames := append([]string(nil), b.columns...)
	rowsData := make([][]any, len(b.rows))
	for index := range b.rows {
		rowsData[index] = append([]any(nil), b.rows[index]...)
	}
	if b.workspaceMode {
		columnNames = append([]string{"workspace_id"}, columnNames...)
		for index := range rowsData {
			rowsData[index] = append([]any{b.workspaceID}, rowsData[index]...)
		}
	}
	context := &renderContext{renderer: b.renderer}
	columns := make([]string, len(columnNames))
	for index, column := range columnNames {
		columns[index] = b.renderer.Identifier(column)
	}
	statement := "INSERT INTO " + b.renderer.Table(b.table) + " (" + strings.Join(columns, ", ") + ") "

	if b.source != nil {
		rendered, err := b.source.render(context, true)
		if err != nil {
			return "", nil, err
		}
		statement += rendered
	} else {
		rows := make([]string, len(rowsData))
		for rowIndex, row := range rowsData {
			if len(row) != len(columnNames) {
				return "", nil, fmt.Errorf("SQL insert row %d has %d values for %d columns", rowIndex, len(row), len(columnNames))
			}
			placeholders := make([]string, len(row))
			for index, value := range row {
				placeholders[index] = context.argument(value)
			}
			rows[rowIndex] = "(" + strings.Join(placeholders, ", ") + ")"
		}
		statement += "VALUES " + strings.Join(rows, ", ")
	}

	if b.conflict != nil {
		clause, err := b.renderConflict(context)
		if err != nil {
			return "", nil, err
		}
		statement += clause
	}
	if len(b.duplicate) > 0 {
		named, ok := b.renderer.(namedRenderer)
		if !ok || named.Name() != dialect.MySQL {
			return "", nil, fmt.Errorf("SQL ON DUPLICATE KEY UPDATE is only supported on MySQL")
		}
		assignments, err := renderAssignments(context, b.duplicate)
		if err != nil {
			return "", nil, err
		}
		statement += " ON DUPLICATE KEY UPDATE " + assignments
	}
	if returning := renderReturning(b.renderer, b.returning); returning != "" {
		statement += returning
	}
	return statement, append([]any(nil), context.args...), nil
}

func (b *InsertBuilder) renderConflict(context *renderContext) (string, error) {
	if named, ok := b.renderer.(namedRenderer); ok && named.Name() == dialect.MySQL {
		if b.conflict.action != conflictDoNothing {
			return "", fmt.Errorf("SQL ON CONFLICT DO UPDATE is not supported on MySQL; use ON DUPLICATE KEY UPDATE")
		}
		if len(b.conflict.targets) == 0 {
			return "", fmt.Errorf("SQL MySQL conflict do nothing requires a target column")
		}
		target := b.renderer.Identifier(b.conflict.targets[0])
		return " ON DUPLICATE KEY UPDATE " + target + " = " + target, nil
	}
	clause := " ON CONFLICT"
	if len(b.conflict.targets) > 0 {
		quoted := make([]string, len(b.conflict.targets))
		for index, target := range b.conflict.targets {
			quoted[index] = b.renderer.Identifier(target)
		}
		clause += " (" + strings.Join(quoted, ", ") + ")"
	}
	switch b.conflict.action {
	case conflictDoNothing:
		return clause + " DO NOTHING", nil
	case conflictDoUpdate:
		if len(b.conflict.assignments) == 0 {
			return "", fmt.Errorf("SQL ON CONFLICT DO UPDATE requires assignments")
		}
		assignments, err := renderAssignments(context, b.conflict.assignments)
		if err != nil {
			return "", err
		}
		clause += " DO UPDATE SET " + assignments
		if b.conflict.predicate != nil {
			where, err := b.conflict.predicate.renderPredicate(context)
			if err != nil {
				return "", err
			}
			clause += " WHERE " + where
		}
		return clause, nil
	default:
		return "", fmt.Errorf("SQL ON CONFLICT requires an action")
	}
}
