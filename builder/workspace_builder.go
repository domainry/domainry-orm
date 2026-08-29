package builder

import (
	"fmt"
	"strings"
)

const WorkspaceIDColumn = "workspace_id"

func workspaceID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("SQL workspace builder requires workspaceID")
	}
	return value, nil
}

// NewWorkspaceSelectBuilder creates a tenant-scoped SELECT. The workspace
// predicate cannot be replaced by a later Where call.
func NewWorkspaceSelectBuilder(renderer Renderer, table, value string) *SelectBuilder {
	builder := NewSelectBuilder(renderer, table)
	builder.workspaceMode = true
	workspace, err := workspaceID(value)
	if err != nil {
		builder.buildError = err
		return builder
	}
	builder.workspaceID = workspace
	return builder
}

// NewWorkspaceInsertBuilder creates a tenant-scoped INSERT and injects the
// workspace_id column/value into every VALUES row.
func NewWorkspaceInsertBuilder(renderer Renderer, table, value string) *InsertBuilder {
	builder := NewInsertBuilder(renderer, table)
	builder.workspaceMode = true
	workspace, err := workspaceID(value)
	if err != nil {
		builder.buildError = err
		return builder
	}
	builder.workspaceID = workspace
	return builder
}

func NewWorkspaceUpdateBuilder(renderer Renderer, table, value string) *UpdateBuilder {
	builder := NewUpdateBuilder(renderer, table)
	workspace, err := workspaceID(value)
	if err != nil {
		builder.buildError = err
		return builder
	}
	builder.required = append(builder.required, Equal(WorkspaceIDColumn, workspace))
	return builder
}

func NewWorkspaceDeleteBuilder(renderer Renderer, table, value string) *DeleteBuilder {
	builder := NewDeleteBuilder(renderer, table)
	workspace, err := workspaceID(value)
	if err != nil {
		builder.buildError = err
		return builder
	}
	builder.required = append(builder.required, Equal(WorkspaceIDColumn, workspace))
	return builder
}
